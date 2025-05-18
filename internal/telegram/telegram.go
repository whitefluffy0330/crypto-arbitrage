package telegram

import (
	"fmt"
	"log"
	"strings"
	"sync" // Імпорт для sync.RWMutex та sync.Mutex
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" // Цей імпорт був у вашому оригіналі, але не використовувався; якщо він потрібен для GetUserGoal, його треба розкоментувати та виправити типи
	gsheets "google.golang.org/api/sheets/v4"
)

// UserState представляє поточний стан діалогу користувача
type UserState string // З вашого оригінального коду

const (
	StateDefault                 UserState = ""
	StateAwaitingGoalInput       UserState = "awaiting_goal_input"
	StateAwaitingInvestmentInput UserState = "awaiting_investment_input"
	StateAwaitingFundingThreshold UserState = "awaiting_funding_threshold"
)

var userStates = make(map[int64]UserState)
var userStatesMutex = &sync.Mutex{} // З вашого оригінального коду userMutex, перейменовано для ясності

// SetUserState встановлює стан для користувача
func SetUserState(chatID int64, state UserState) {
	userStatesMutex.Lock()
	defer userStatesMutex.Unlock()
	userStates[chatID] = state
	log.Printf("Встановлено стан '%s' для ChatID %d", state, chatID) // Додано логування
}

// GetUserState повертає стан для користувача
func GetUserState(chatID int64) UserState {
	userStatesMutex.Lock()
	defer userStatesMutex.Unlock()
	state, exists := userStates[chatID] // Змінено для перевірки існування
	if !exists {
		return StateDefault
	}
	return state
}

// FinancialGoal - визначення з вашого оригінального файлу telegram.go (якщо він там був)
// Або з файлу goal.go, якщо ви його використовуєте.
// Для прикладу, я використовую визначення, яке ми обговорювали раніше.
type FinancialGoal struct {
	Amount       float64
	Currency     string
	Days         int // Можливо, не використовується для місячних цілей
	OriginalText string
	SetDate      time.Time
}

var (
	userGoals      = make(map[int64]FinancialGoal)
	userGoalsMutex sync.RWMutex
)


var userFundingThresholds = make(map[int64]float64)
var fundingThresholdMutex = &sync.Mutex{} // З вашого оригінального коду

const defaultFundingThreshold = 0.0005

func SetUserFundingThreshold(chatID int64, threshold float64) {
	fundingThresholdMutex.Lock()
	defer fundingThresholdMutex.Unlock()
	userFundingThresholds[chatID] = threshold
	log.Printf("Встановлено поріг фандингу %.4f%% для ChatID %d", threshold*100, chatID)
}

func GetUserFundingThreshold(chatID int64) float64 {
	fundingThresholdMutex.Lock()
	defer fundingThresholdMutex.Unlock()
	if threshold, ok := userFundingThresholds[chatID]; ok {
		log.Printf("Для ChatID %d використовується поріг фандингу %.4f%%", chatID, threshold*100)
		return threshold
	}
	log.Printf("Для ChatID %d поріг фандингу не встановлено, використовується стандартний %.4f%%", chatID, defaultFundingThreshold*100)
	return defaultFundingThreshold
}

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
	userStates = make(map[int64]UserState)
	userFundingThresholds = make(map[int64]float64)
	userGoals = make(map[int64]FinancialGoal) // Ініціалізація userGoals
}

// InitBot ініціалізує та повертає екземпляр бота.
// Змінена логіка перевірки bot.Self для уникнення помилки "mismatched types"
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

	if bot == nil {
		log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: bot є nil після NewBotAPI, хоча помилки не було.")
		return nil, fmt.Errorf("bot is nil after NewBotAPI without an error")
	}
	
	// Обережна перевірка bot.Self.ID. Якщо GetMe потрібен, його треба викликати обережно.
	// Оригінальний код мав `if bot.Self.ID == 0`, що могло викликати паніку, якщо `bot.Self` nil.
	// Попередня спроба `if bot.Self == nil` викликала помилку компіляції.
	// Спробуємо викликати GetMe, якщо bot.Self виглядає неініціалізованим.
	if bot.Self == nil || bot.Self.ID == 0 { // <--- Змінено для більшої обережності
		log.Printf("ПОПЕРЕДЖЕННЯ: bot.Self є nil або bot.Self.ID = 0. Спроба викликати GetMe().")
		userInfo, errGetMe := bot.GetMe()
		if errGetMe != nil {
			log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: GetMe() повернув помилку: %v. Можливо, невалідний токен.", errGetMe)
			return nil, fmt.Errorf("GetMe failed after NewBotAPI: %w. Token might be invalid", errGetMe)
		}
		if userInfo.ID == 0 {
			log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: GetMe() повернув userInfo.ID = 0. Непередбачена ситуація.")
			return nil, fmt.Errorf("GetMe returned userInfo.ID 0, unexpected")
		}
		bot.Self = userInfo // Оновлюємо інформацію про бота
		log.Printf("Інформацію про бота отримано через GetMe(): @%s (ID: %d)", bot.Self.UserName, bot.Self.ID)
	} else {
		log.Printf("Бот успішно ініціалізований: ID=%d, UserName='%s'", bot.Self.ID, bot.Self.UserName)
	}
	return bot, nil
}

// HandleUpdates (якщо ця функція тут, а не в handler.go)
func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) {
	log.Println("Розпочато обробку оновлень Telegram...")
	for update := range updates {
		go HandleUpdate(bot, update, srv, cfg)
	}
	log.Println("Зупинено обробку оновлень Telegram (канал закрито).")
}

// SetWebhook встановлює вебхук для бота.
// Ця версія ВІДПОВІДАЄ ОРИГІНАЛЬНОМУ КОДУ з коміту c230a60,
// де очікується, що NewWebhook... повертає (WebhookConfig, error).
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error {
	log.Printf("Встановлення вебхука: URL=%s%s, CertFile (якщо є)=%s", webhookBaseURL, webhookPath, certFilePath)
	fullWebhookURL := webhookBaseURL + webhookPath
	if !strings.HasPrefix(fullWebhookURL, "https://") && webhookBaseURL != "" { // Додано перевірку, що webhookBaseURL не порожній
		log.Printf("ПОПЕРЕДЖЕННЯ: URL вебхука '%s' не починається з https://.", fullWebhookURL)
	}

	// Якщо WebhookBaseURL порожній, вебхук не встановлюємо (як у вашому оригінальному main.go)
	if webhookBaseURL == "" || webhookPath == "" {
		log.Println("ПОПЕРЕДЖЕННЯ: WebhookBaseURL або WebhookPath не вказані. Вебхук не буде встановлено.")
		// Можна спробувати видалити існуючий вебхук, щоб уникнути проблем
		// removeErr := RemoveWebhook(bot)
		// if removeErr != nil {
		// 	log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося видалити існуючий вебхук при порожньому URL: %v", removeErr)
		// }
		return nil // Не встановлюємо вебхук, якщо немає URL/Path
	}

	var whCfg tgbotapi.WebhookConfig
	var errWebhookSetup error // Повертаємо цю змінну

	if certFilePath != "" {
		log.Printf("Спроба встановити вебхук з файлом сертифіката: %s", certFilePath)
		fileBytes := tgbotapi.FilePath(certFilePath)
		whCfg, errWebhookSetup = tgbotapi.NewWebhookWithCert(fullWebhookURL, fileBytes) // Очікуємо два значення
	} else {
		log.Printf("Спроба встановити вебхук без файлу сертифіката.")
		whCfg, errWebhookSetup = tgbotapi.NewWebhook(fullWebhookURL) // Очікуємо два значення
	}

	if errWebhookSetup != nil {
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


// ... (RemoveWebhook, SetUserGoal, GetUserGoal, DeleteUserGoal, ClearInMemoryUserGoal залишаються без змін відносно відповіді #51, тут вони для повноти)

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

// Заглушка для HandleUpdate
// ВАЖЛИВО: Якщо у вас є файл handler.go з функцією HandleUpdate, то ця заглушка тут не потрібна,
// а функція HandleUpdates вище має викликати handler.HandleUpdate.
// Якщо ж вся логіка обробки має бути тут, то розкоментуйте та доповніть.
// Поки що, для уникнення помилки "redeclared", я залишаю її закоментованою,
// припускаючи, що реальна HandleUpdate є в handler.go.
/*
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	log.Printf("Отримано оновлення ID: %d (з telegram.go - ЗАГЛУШКА)", update.UpdateID)
	if update.Message != nil {
		log.Printf("Повідомлення від %s: %s", update.Message.From.UserName, update.Message.Text)
	}
}
*/
// Якщо компілятор скаржиться на відсутність HandleUpdate, розкоментуйте блок вище, АЛЕ
// переконайтеся, що у вас немає іншої функції HandleUpdate в цьому ж пакеті (наприклад, у handler.go).
// Якщо є, то потрібно або перейменувати одну з них, або об'єднати логіку.
// Згідно з останньою помилкою, у вас є HandleUpdate в handler.go, тому ця заглушка не потрібна.
