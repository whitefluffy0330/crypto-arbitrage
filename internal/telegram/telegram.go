package telegram

import (
	"fmt"
	"log"
	"strings"
	"sync" // Потрібен для userMutex та fundingThresholdMutex
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // Потрібен для KyivLocation у sheets та інших функцій
	gsheets "google.golang.org/api/sheets/v4"                     // Аліас для офіційного пакета Sheets
)

// UserState представляє поточний стан діалогу користувача
type UserState string // Змінено з константи на тип для більшої гнучкості, якщо потрібно

const (
	StateDefault                 UserState = "" // Використовуємо тип UserState
	StateAwaitingGoalInput       UserState = "awaiting_goal_input"
	StateAwaitingInvestmentInput UserState = "awaiting_investment_input"
	StateAwaitingFundingThreshold UserState = "awaiting_funding_threshold"
)

var userStates = make(map[int64]UserState)

// userMutex був визначений у вашому оригінальному коді, але не використовувався.
// Якщо він потрібен для userStates, потрібно додати блокування.
// Для простоти поки що без нього, якщо не було явних проблем.
// Якщо userStates модифікується з різних горутин, userMutex потрібен.
// Згідно з вашим оригінальним кодом, userMutex був &sync.Mutex{}
var userStatesMutex sync.RWMutex // Використовуємо RWMutex, як для userGoals

// FinancialGoal - структура для зберігання інформації про фінансову ціль користувача.
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

var userFundingThresholds = make(map[int64]float64)
var fundingThresholdMutex sync.RWMutex // Використовуємо RWMutex, як для userGoals

const defaultFundingThreshold = 0.0005

var KyivLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Printf("Помилка завантаження часової зони Europe/Kyiv: %v. Використовується UTC.", err)
		KyivLocation = time.UTC
	} else {
		KyivLocation = loc
		log.Println("Часову зону Europe/Kyiv завантажено (telegram).")
	}
	// Ініціалізація userStates та userFundingThresholds, якщо вони не ініціалізуються при оголошенні
	// userStates = make(map[int64]UserState) // Вже зроблено при оголошенні
	// userFundingThresholds = make(map[int64]float64) // Вже зроблено при оголошенні
}

// SetUserState встановлює стан для користувача
func SetUserState(chatID int64, state UserState) {
	userStatesMutex.Lock() // Використовуємо RWMutex
	defer userStatesMutex.Unlock()
	if state == StateDefault {
		delete(userStates, chatID)
	} else {
		userStates[chatID] = state
	}
	log.Printf("Встановлено стан '%s' для ChatID %d", state, chatID)
}

// GetUserState повертає стан для користувача
func GetUserState(chatID int64) UserState {
	userStatesMutex.RLock() // Використовуємо RWMutex
	defer userStatesMutex.RUnlock()
	state, exists := userStates[chatID]
	if !exists {
		return StateDefault
	}
	return state
}

func SetUserFundingThreshold(chatID int64, threshold float64) {
	fundingThresholdMutex.Lock()
	defer fundingThresholdMutex.Unlock()
	userFundingThresholds[chatID] = threshold
	log.Printf("Встановлено поріг фандингу %.4f%% для ChatID %d", threshold*100, chatID)
}

func GetUserFundingThreshold(chatID int64) float64 {
	fundingThresholdMutex.RLock() // Використовуємо RWMutex
	defer fundingThresholdMutex.RUnlock()
	if threshold, ok := userFundingThresholds[chatID]; ok {
		log.Printf("Для ChatID %d використовується поріг фандингу %.4f%%", chatID, threshold*100)
		return threshold
	}
	log.Printf("Для ChatID %d поріг фандингу не встановлено, використовується стандартний %.4f%%", chatID, defaultFundingThreshold*100)
	return defaultFundingThreshold
}


// InitBot ініціалізує та повертає екземпляр бота.
// Повертаємося до логіки перевірки ID, припускаючи, що bot та bot.Self не nil, якщо NewBotAPI не повернув помилку.
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

	// Якщо NewBotAPI не повернув помилку, вважаємо, що bot != nil
	// Якщо bot.Self буде nil, то bot.Self.ID викличе паніку.
	// Ця логіка ближча до вашого оригінального коду.
	// Якщо компілятор все ще скаржиться на "mismatched types" для "bot.Self == nil" в інших місцях,
	// то це дійсно проблема інтерпретації типів.
	if bot.Self.ID == 0 { // <--- Оригінальна перевірка (або близька до неї)
		// Спробуємо обережно отримати UserName, якщо Self не nil, але ID = 0
		var userNameForLog string
		if bot.Self != nil { // Ця перевірка може викликати помилку компіляції, якщо проблема з типом
			userNameForLog = bot.Self.UserName
		} else {
			userNameForLog = "[bot.Self є nil!]"
			// Це критична ситуація, якщо bot.Self nil, а NewBotAPI не повернув помилку.
			log.Printf("КРИТИЧНА ПОМИЛКА: bot.Self є nil після успішного NewBotAPI!")
			return nil, fmt.Errorf("bot.Self is nil after NewBotAPI success, this should not happen")
		}
		log.Printf("ПОПЕРЕДЖЕННЯ: bot.Self.ID = 0 після NewBotAPI. UserName: '%s'. Перевірте токен.", userNameForLog)
	} else {
		log.Printf("Бот успішно ініціалізований: ID=%d, UserName='%s'", bot.Self.ID, bot.Self.UserName)
	}
	return bot, nil
}


// ... (функції SetUserGoal, GetUserGoal, DeleteUserGoal, ClearInMemoryUserGoal залишаються як у відповіді №46)
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

func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) {
	log.Println("Розпочато обробку оновлень Telegram...")
	for update := range updates {
		go HandleUpdate(bot, update, srv, cfg)
	}
	log.Println("Зупинено обробку оновлень Telegram (канал закрито).")
}

// SetWebhook встановлює вебхук для бота.
// ЦЯ ВЕРСІЯ ПОВЕРТАЄТЬСЯ ДО ОЧІКУВАННЯ ДВОХ ЗНАЧЕНЬ ВІД NewWebhook...
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
	var errWebhookSetup error // <--- ПОВЕРТАЄМО ЗМІННУ ДЛЯ ПОМИЛКИ

	if certFilePath != "" {
		log.Printf("Спроба встановити вебхук з файлом сертифіката: %s", certFilePath)
		fileBytes := tgbotapi.FilePath(certFilePath)
		whCfg, errWebhookSetup = tgbotapi.NewWebhookWithCert(fullWebhookURL, fileBytes) // <--- ОЧІКУЄМО ДВА ЗНАЧЕННЯ
	} else {
		log.Printf("Спроба встановити вебхук без файлу сертифіката (Nginx має обробляти SSL).")
		whCfg, errWebhookSetup = tgbotapi.NewWebhook(fullWebhookURL) // <--- ОЧІКУЄМО ДВА ЗНАЧЕННЯ
	}

	if errWebhookSetup != nil { // <--- ОБРОБЛЯЄМО ПОМИЛКУ ВІД NewWebhook...
		log.Printf("ПОМИЛКА конфігурації вебхука при виклику NewWebhook...: %v", errWebhookSetup)
		return fmt.Errorf("помилка конфігурації вебхука NewWebhook...: %w", errWebhookSetup)
	}

	whCfg.MaxConnections = 40
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

// HandleUpdate обробляє окреме оновлення.
// Ця функція має бути реалізована у вашому файлі handler.go або тут, якщо вона проста.
// Для прикладу, я додам заглушку.
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	// Тут буде ваша логіка обробки команд, повідомлень, callback'ів
	// Наприклад, виклик функції з handler.go
	// telegramHandler.HandleUpdate(bot, update, srv, cfg) // Якщо у вас є такий об'єкт/пакет
	log.Printf("Отримано оновлення: %+v", update.UpdateID)

	// Перевірка, чи це повідомлення, і чи є текст
	if update.Message != nil && update.Message.Text != "" {
		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
		// msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
		// bot.Send(msg)
	}
}
