package telegram

import (
	"fmt"
	"log"
	"strings"
	"sync" // Був відсутній у вашому оригінальному telegram.go, але потрібен для м'ютексів
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4" // Додано аліас для офіційного пакета
)

var KyivLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Printf("Крит. помилка telegram: не вдалося завантажити часову зону 'Europe/Kyiv': %v.", err)
		KyivLocation = time.UTC
	} else {
		KyivLocation = loc
		log.Println("Часову зону Europe/Kyiv завантажено (telegram).")
	}
}

type FinancialGoal struct {
	Amount       float64
	Currency     string
	Days         int
	OriginalText string
	SetDate      time.Time
}

var (
	userGoals      = make(map[int64]FinancialGoal)
	userGoalsMutex sync.RWMutex // Змінено на RWMutex для консистентності з іншими
)

var (
	userStates      = make(map[int64]string) // Використовуємо string, як у вашому оригінальному коді
	userStatesMutex sync.RWMutex // Змінено на RWMutex
)

const (
	StateDefault                 = ""
	StateAwaitingGoalInput       = "awaiting_goal_input"
	StateAwaitingInvestmentInput = "awaiting_investment_input"
	StateAwaitingFundingThreshold = "awaiting_funding_threshold"
)

var (
	userFundingThresholds      = make(map[int64]float64)
	userFundingThresholdsMutex sync.RWMutex // Змінено на RWMutex
)

const defaultFundingThreshold = 0.0005

func SetUserState(chatID int64, state string) {
	userStatesMutex.Lock()
	defer userStatesMutex.Unlock()
	if state == StateDefault {
		delete(userStates, chatID)
	} else {
		userStates[chatID] = state
	}
	log.Printf("Встановлено стан '%s' для ChatID %d", state, chatID)
}

func GetUserState(chatID int64) string {
	userStatesMutex.RLock()
	defer userStatesMutex.RUnlock()
	state, exists := userStates[chatID]
	if !exists {
		return StateDefault
	}
	return state
}

func SetUserGoal(chatID int64, goal FinancialGoal, srv *gsheets.Service, cfg config.Config) error {
	activeGoal, isActive := GetUserGoal(chatID, srv, cfg)
	if isActive {
		log.Printf("Для ChatID %d знайдено активну ціль (%+v) перед встановленням нової. Оновлюємо її статус на 'Перевизначено'.", chatID, activeGoal)
		errUpdateOld := sheets.UpdateGoalStatusInSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, "Перевизначено", time.Now().UTC())
		if errUpdateOld != nil {
			log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося оновити статус старої активної цілі на 'Перевизначено' для ChatID %d: %v", chatID, errUpdateOld)
		}
	}

	goalDataForSheet := sheets.FinancialGoalData{
		Amount:       goal.Amount,
		Currency:     goal.Currency,
		Days:         0, // Це поле було у вашому оригіналі, залишаємо
		OriginalText: goal.OriginalText,
		SetDate:      goal.SetDate,
	}
	err := sheets.AddGoalToSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, goalDataForSheet)
	if err != nil {
		log.Printf("ПОМИЛКА запису нової цілі в Sheets для ChatID %d: %v", chatID, err)
		return fmt.Errorf("збереження нової цілі в Sheets: %w", err)
	}

	userGoalsMutex.Lock()
	userGoals[chatID] = goal
	userGoalsMutex.Unlock()
	log.Printf("Нову ціль успішно збережено та закешовано для ChatID %d: %+v", chatID, goal)
	return nil
}

func GetUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) (FinancialGoal, bool) {
	userGoalsMutex.RLock()
	goal, exists := userGoals[chatID]
	userGoalsMutex.RUnlock()
	if exists {
		return goal, true
	}

	log.Printf("Активна ціль для ChatID %d не знайдена в кеші. Спроба завантажити з Google Sheets (аркуш: %s).", chatID, cfg.SheetNameUserGoals)
	sheetGoalData, foundInSheet, err := sheets.GetActiveGoalFromSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID)
	if err != nil {
		log.Printf("ПОМИЛКА завантаження активної цілі з Sheets для ChatID %d: %v", chatID, err)
		return FinancialGoal{}, false
	}
	if foundInSheet {
		loadedGoal := FinancialGoal{
			Amount:       sheetGoalData.Amount,
			Currency:     sheetGoalData.Currency,
			Days:         sheetGoalData.Days,
			OriginalText: sheetGoalData.OriginalText,
			SetDate:      sheetGoalData.SetDate,
		}
		userGoalsMutex.Lock()
		userGoals[chatID] = loadedGoal
		userGoalsMutex.Unlock()
		log.Printf("Активну ціль для ChatID %d успішно завантажено з Sheets та закешовано: %+v", chatID, loadedGoal)
		return loadedGoal, true
	}
	log.Printf("Активну ціль для ChatID %d не знайдено ні в кеші, ні в Sheets.", chatID)
	return FinancialGoal{}, false
}

func DeleteUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) error {
	log.Printf("Спроба закрити активну ціль для ChatID %d.", chatID)
	err := sheets.UpdateGoalStatusInSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, "Закрита", time.Now().UTC())
	if err != nil {
		if strings.Contains(err.Error(), "не знайдено активної цілі") {
			log.Printf("Немає активної цілі в Google Sheets для ChatID %d, щоб позначити як 'Закрита'.", chatID)
			ClearInMemoryUserGoal(chatID)
			return nil
		}
		log.Printf("ПОМИЛКА оновлення статусу цілі в Sheets для ChatID %d: %v", chatID, err)
		ClearInMemoryUserGoal(chatID)
		return fmt.Errorf("помилка оновлення статусу цілі в Google Sheets: %w", err)
	}
	ClearInMemoryUserGoal(chatID)
	log.Printf("Активну ціль для ChatID %d успішно позначено як 'Закрита' в Sheets та видалено з кешу.", chatID)
	return nil
}

func ClearInMemoryUserGoal(chatID int64) {
	userGoalsMutex.Lock()
	defer userGoalsMutex.Unlock()
	if _, exists := userGoals[chatID]; exists {
		delete(userGoals, chatID)
		log.Printf("Ціль для ChatID %d видалено з кешу.", chatID)
	} else {
		log.Printf("Ціль для ChatID %d вже відсутня в кеші, нічого видаляти.", chatID)
	}
}

func SetUserFundingThreshold(chatID int64, threshold float64) {
	userFundingThresholdsMutex.Lock()
	defer userFundingThresholdsMutex.Unlock()
	userFundingThresholds[chatID] = threshold
	log.Printf("Встановлено поріг фандингу %.4f%% для ChatID %d", threshold*100, chatID)
}

func GetUserFundingThreshold(chatID int64) float64 {
	userFundingThresholdsMutex.RLock()
	defer userFundingThresholdsMutex.RUnlock()
	threshold, exists := userFundingThresholds[chatID]
	if !exists {
		log.Printf("Для ChatID %d поріг фандингу не встановлено, використовується стандартний %.4f%%", chatID, defaultFundingThreshold*100)
		return defaultFundingThreshold
	}
	log.Printf("Для ChatID %d використовується поріг фандингу %.4f%%", chatID, threshold*100)
	return threshold
}

// InitBot ініціалізує та повертає екземпляр бота.
func InitBot(token string) (*tgbotapi.BotAPI, error) {
	log.Println("Спроба ініціалізації бота через tgbotapi.NewBotAPI...")
	if token == "" {
		return nil, fmt.Errorf("токен бота порожній, перевірте змінну середовища TELEGRAM_TOKEN")
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Помилка tgbotapi.NewBotAPI: %v", err)
		return nil, fmt.Errorf("не вдалося створити BotAPI: %w", err)
	}

	// Якщо NewBotAPI не повернув помилку, вважаємо, що bot != nil.
	// Якщо bot.Self буде nil, наступний рядок викличе паніку.
	// Це відповідає логіці вашого оригінального коду з коміту c230a60.
	// Якщо компілятор скаржиться на "mismatched types tgbotapi.User and untyped nil" 
	// при спробі додати "if bot.Self == nil", то це проблема інтерпретації типів.
	if bot.Self.ID == 0 {
		log.Printf("ПОПЕРЕДЖЕННЯ: bot.Self.ID = 0 після NewBotAPI. UserName: '%s'. Перевірте токен.", bot.Self.UserName)
	} else {
		log.Printf("Бот успішно ініціалізований: ID=%d, UserName='%s'", bot.Self.ID, bot.Self.UserName)
	}
	return bot, nil
}

func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) {
	log.Println("Розпочато обробку оновлень Telegram...")
	for update := range updates {
		go HandleUpdate(bot, update, srv, cfg) // Передаємо HandleUpdate з цього ж пакету
	}
	log.Println("Зупинено обробку оновлень Telegram (канал закрито).")
}

// SetWebhook встановлює вебхук для бота.
// Ця версія ВІДПОВІДАЄ ОРИГІНАЛЬНОМУ КОДУ з коміту c230a60,
// де очікується, що NewWebhook... повертає (WebhookConfig, error).
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error {
	log.Printf("Встановлення вебхука: URL=%s%s, CertFile (якщо є)=%s", webhookBaseURL, webhookPath, certFilePath)
	fullWebhookURL := webhookBaseURL + webhookPath
	if !strings.HasPrefix(fullWebhookURL, "https://") {
		log.Printf("ПОПЕРЕДЖЕННЯ: URL вебхука '%s' не починається з https://.", fullWebhookURL)
	}

	var whCfg tgbotapi.WebhookConfig
	var errWebhookSetup error // Змінна для можливої помилки від NewWebhook...

	if certFilePath != "" {
		log.Printf("Спроба встановити вебхук з файлом сертифіката: %s", certFilePath)
		fileBytes := tgbotapi.FilePath(certFilePath)
		whCfg, errWebhookSetup = tgbotapi.NewWebhookWithCert(fullWebhookURL, fileBytes)
	} else {
		log.Printf("Спроба встановити вебхук без файлу сертифіката.")
		whCfg, errWebhookSetup = tgbotapi.NewWebhook(fullWebhookURL)
	}

	if errWebhookSetup != nil {
		log.Printf("ПОМИЛКА конфігурації вебхука при виклику NewWebhook...: %v", errWebhookSetup)
		return fmt.Errorf("помилка конфігурації вебхука NewWebhook...: %w", errWebhookSetup)
	}

	whCfg.MaxConnections = 40 // Це було у вашому оригінальному коді
	_, err := bot.Request(whCfg) 
	if err != nil {
		log.Printf("ПОМИЛКА встановлення вебхука '%s': %v", fullWebhookURL, err)
		return fmt.Errorf("bot.Request(webhook setup) failed: %w", err)
	}

	webhookInfo, errInfo := bot.GetWebhookInfo() 
	if errInfo != nil {
		log.Printf("ПОМИЛКА отримання інформації про вебхук після встановлення: %v", errInfo)
		return fmt.Errorf("bot.GetWebhookInfo failed after setup: %w", errInfo)
	}

	if webhookInfo.IsSet() && webhookInfo.URL == fullWebhookURL {
		log.Printf("Вебхук успішно встановлено: URL='%s', MaxConnections=%d, PendingUpdates=%d",
			webhookInfo.URL, webhookInfo.MaxConnections, webhookInfo.PendingUpdateCount)
		if webhookInfo.LastErrorDate != 0 {
			log.Printf("  Остання помилка вебхука: %s (дата: %s)", webhookInfo.LastErrorMessage, time.Unix(int64(webhookInfo.LastErrorDate), 0).Format(time.RFC3339))
		}
		if webhookInfo.IPAddress != "" {
			log.Printf("  IP-адреса вебхука (як бачить Telegram): %s", webhookInfo.IPAddress)
		}
	} else {
		log.Printf("ПОПЕРЕДЖЕННЯ: Вебхук НЕ встановлено належним чином після запиту. Поточний URL: '%s', Очікуваний: '%s'", webhookInfo.URL, fullWebhookURL)
		return fmt.Errorf("webhook URL ('%s') was not set to the expected ('%s') by Telegram after request", webhookInfo.URL, fullWebhookURL)
	}
	return nil
}

func RemoveWebhook(bot *tgbotapi.BotAPI) error {
	log.Println("Спроба видалення вебхука...")
	_, err := bot.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: true})
	if err != nil {
		log.Printf("ПОМИЛКА видалення вебхука: %v", err)
		return fmt.Errorf("bot.Request(delete webhook) failed: %w", err)
	}
	log.Println("Запит на видалення вебхука надіслано.")

	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Printf("ПОМИЛКА отримання інформації про вебхук після запиту на видалення: %v", err)
	} else if info.IsSet() && info.URL != "" {
		log.Printf("ПОПЕРЕДЖЕННЯ: Вебхук все ще встановлений на URL: '%s' після запиту на видалення.", info.URL)
	} else {
		log.Println("Вебхук успішно видалено (або не був встановлений).")
	}
	return nil
}

// Заглушка для HandleUpdate, якщо основна логіка в handler.go
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	log.Printf("Отримано оновлення ID: %d (з telegram.go)", update.UpdateID)
	// Тут має бути виклик функції з вашого пакету handler, наприклад:
	// handler.ProcessUpdate(bot, update, srv, cfg)
	// Або, якщо ваш handler.go містить функцію з такою ж назвою HandleUpdate,
	// вам потрібно буде перейменувати одну з них або використовувати різні пакети.
	// Поки що, для уникнення конфлікту імен, якщо у вас є handler.HandleUpdate,
	// цю функцію можна було б назвати інакше або вона має викликати ту.
	// Для простоти, я залишу тільки логування.
	if update.Message != nil {
		log.Printf("Повідомлення від %s: %s", update.Message.From.UserName, update.Message.Text)
	}
}
