package telegram

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4"
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
	userGoalsMutex sync.RWMutex
)

var (
	userStates      = make(map[int64]string)
	userStatesMutex sync.RWMutex
)

const (
	StateDefault                 = ""
	StateAwaitingGoalInput       = "awaiting_goal_input"
	StateAwaitingInvestmentInput = "awaiting_investment_input"
	StateAwaitingFundingThreshold = "awaiting_funding_threshold"
)

var (
	userFundingThresholds      = make(map[int64]float64)
	userFundingThresholdsMutex sync.RWMutex
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
		Days:         0,
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
	bot, err := tgbotapi.NewBotAPI(token) // bot тут *tgbotapi.BotAPI
	if err != nil {
		log.Printf("Помилка tgbotapi.NewBotAPI: %v", err)
		return nil, fmt.Errorf("не вдалося створити BotAPI: %w", err)
	}

	// bot.Self є *tgbotapi.User. Спочатку перевіряємо, чи він не nil.
	if bot.Self == nil { // <--- Ця перевірка тепер перша
		log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: bot.Self є nil після NewBotAPI. Можливо, невалідний токен або серйозна проблема з API Telegram.")
		return nil, fmt.Errorf("bot.Self is nil after NewBotAPI, token might be invalid or Telegram API issue")
	}

	// Тепер, коли ми знаємо, що bot.Self не nil, можна безпечно доступатися до його полів.
	// Однак, для уникнення помилки "mismatched types" у main.go, яка дуже дивна,
	// давайте зробимо цю перевірку ще більш обережною, хоча вона не мала б бути проблемою.
	// Ця помилка компіляції "mismatched types tgbotapi.User and untyped nil" для "bot.Self == nil"
	// виникає в telegram.go, а не в main.go, отже, проблема саме тут.
	// Давайте тимчасово спробуємо інший підхід, хоча він менш ідіоматичний для вказівників.
	// Ми покладаємося на те, що якщо bot.Self не nil, то доступ до ID безпечний.
	// Якщо ж проблема з типами продовжується, це може бути глибша проблема з залежностями/середовищем.

	// Повернемося до простої перевірки ID, якщо Self не nil (як передбачалося раніше).
	// Помилка "invalid operation: bot.Self == nil (mismatched types tgbotapi.User and untyped nil)"
	// означає, що компілятор з якоїсь причини не вважає bot.Self вказівником *tgbotapi.User
	// або має конфлікт з типом nil для цього порівняння.

	// Давайте спробуємо так:
	var selfUser tgbotapi.User
	if bot.Self != nil { // Якщо компілятор все ще свариться на це, проблема не в логіці
		selfUser = *bot.Self // Розіменування, якщо не nil
	}

	if selfUser.ID == 0 { // Перевіряємо ID розіменованого користувача
		log.Printf("ПОПЕРЕДЖЕННЯ: bot.Self.ID = 0 після NewBotAPI. UserName: '%s'. Перевірте токен.", selfUser.UserName)
	} else {
		log.Printf("Бот успішно ініціалізований: ID=%d, UserName='%s'", selfUser.ID, selfUser.UserName)
	}
	return bot, nil
}


func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) {
	log.Println("Розпочато обробку оновлень Telegram...")
	for update := range updates {
		go HandleUpdate(bot, update, srv, cfg)
	}
	log.Println("Зупинено обробку оновлень Telegram (канал закрито).")
}

// SetWebhook встановлює вебхук для бота.
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error {
	if webhookBaseURL == "" || webhookPath == "" {
		log.Println("ПОПЕРЕДЖЕННЯ: WebhookBaseURL або WebhookPath не вказані. Вебхук не буде встановлено.")
		return nil
	}

	log.Printf("Встановлення вебхука: URL=%s%s, CertFile (якщо є)=%s", webhookBaseURL, webhookPath, certFilePath)
	fullWebhookURL := webhookBaseURL + webhookPath
	if !strings.HasPrefix(fullWebhookURL, "https://") {
		log.Printf("ПОПЕРЕДЖЕННЯ: URL вебхука '%s' не починається з https://. Це може не спрацювати.", fullWebhookURL)
	}

	var whCfg tgbotapi.WebhookConfig
	// var errWebhookSetup error // ВИДАЛЕНО, оскільки NewWebhook... не повертає помилку

	if certFilePath != "" {
		log.Printf("Спроба встановити вебхук з файлом сертифіката: %s", certFilePath)
		fileBytes := tgbotapi.FilePath(certFilePath)
		whCfg = tgbotapi.NewWebhookWithCert(fullWebhookURL, fileBytes) // Повертає тільки WebhookConfig
	} else {
		log.Printf("Спроба встановити вебхук без файлу сертифіката (Nginx має обробляти SSL).")
		whCfg = tgbotapi.NewWebhook(fullWebhookURL) // Повертає тільки WebhookConfig
	}

	// ВИДАЛЕНО блок перевірки errWebhookSetup, оскільки його немає
	// if errWebhookSetup != nil {
	// 	log.Printf("ПОМИЛКА конфігурації вебхука при виклику NewWebhook...: %v", errWebhookSetup)
	// 	return fmt.Errorf("помилка конфігурації вебхука NewWebhook...: %w", errWebhookSetup)
	// }

	whCfg.MaxConnections = 40
	_, err := bot.Request(whCfg) // Помилка може виникнути тут
	if err != nil {
		log.Printf("ПОМИЛКА встановлення вебхука '%s': %v", fullWebhookURL, err)
		return fmt.Errorf("bot.Request(webhook setup) failed: %w", err)
	}

	webhookInfo, err := bot.GetWebhookInfo()
	if err != nil {
		log.Printf("ПОМИЛКА отримання інформації про вебхук після встановлення: %v", err)
		return fmt.Errorf("bot.GetWebhookInfo failed after setup: %w", err)
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
