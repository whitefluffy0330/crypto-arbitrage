package telegram

import (
	"fmt"
	"log"
	"strings"
	"sync" // Імпорт для sync.Mutex та sync.RWMutex
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4"
)

// UserState представляє поточний стан діалогу користувача
type UserState string

const (
	StateDefault                 UserState = ""
	StateAwaitingGoalInput       UserState = "awaiting_goal_input"
	StateAwaitingInvestmentInput UserState = "awaiting_investment_input"
	StateAwaitingFundingThreshold UserState = "awaiting_funding_threshold"
)

var userStates = make(map[int64]UserState)
var userStatesMutex = &sync.Mutex{} // З вашого коду

func SetUserState(chatID int64, state UserState) {
	userStatesMutex.Lock()
	defer userStatesMutex.Unlock()
	userStates[chatID] = state
	log.Printf("Встановлено стан '%s' для ChatID %d", state, chatID)
}

func GetUserState(chatID int64) UserState {
	userStatesMutex.Lock()
	defer userStatesMutex.Unlock()
	state, exists := userStates[chatID]
	if !exists {
		return StateDefault
	}
	return state
}

var userFundingThresholds = make(map[int64]float64)
var fundingThresholdMutex = &sync.Mutex{} // З вашого коду

const defaultFundingThreshold = 0.0005

func SetUserFundingThreshold(chatID int64, threshold float64) {
	fundingThresholdMutex.Lock()
	defer fundingThresholdMutex.Unlock()
	userFundingThresholds[chatID] = threshold
	log.Printf("Встановлено поріг фандингу %.4f%% для ChatID %d", threshold*100, chatID)
}

func GetUserFundingThreshold(chatID int64) float64 {
	fundingThresholdMutex.Lock() // Змінено на Lock для консистентності, як у вас було
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
	userStates = make(map[int64]UserState) // Ініціалізація з вашого коду
	userFundingThresholds = make(map[int64]float64) // Ініціалізація з вашого коду
}

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
	if bot == nil { // Додаткова перевірка, якщо NewBotAPI не повернув помилку, але bot - nil
		log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: bot є nil після NewBotAPI, хоча помилки не було.")
		return nil, fmt.Errorf("bot is nil after NewBotAPI without an error")
	}

	// Використовуємо логіку, максимально наближену до вашої "вчорашньої робочої версії",
	// де був прямий доступ до bot.Self.ID. Це ризиковано, якщо bot.Self може бути nil
	// і компілятор бачить bot.Self як *User.
	// Якщо компілятор бачить bot.Self як User (не вказівник), то доступ до ID коректний,
	// але порівняння bot.Self == nil викличе помилку "mismatched types".
	// Цей код має компілюватися, якщо компілятор бачить bot.Self як User.
	if bot.Self.ID == 0 {
		// Спроба викликати GetMe для отримання інформації, якщо ID нульовий
		log.Printf("ПОПЕРЕДЖЕННЯ: bot.Self.ID = 0 після NewBotAPI. UserName з Self: '%s'. Спроба GetMe().", bot.Self.UserName)
		userInfo, errGetMe := bot.GetMe() // userInfo тут завжди *tgbotapi.User
		if errGetMe != nil {
			log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: GetMe() повернув помилку: %v. Можливо, невалідний токен.", errGetMe)
			return nil, fmt.Errorf("GetMe failed after NewBotAPI: %w. Token might be invalid", errGetMe)
		}
		if userInfo == nil || userInfo.ID == 0 {
			log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: GetMe() повернув nil або userInfo.ID = 0.")
			return nil, fmt.Errorf("GetMe returned nil or zero ID user")
		}
		// Оскільки ми не знаємо точний тип bot.Self (User чи *User з точки зору компілятора),
		// ми не будемо намагатися присвоїти userInfo до bot.Self тут, щоб уникнути нових помилок типів.
		// Просто логуємо отриману інформацію.
		log.Printf("Інформацію про бота отримано через GetMe(): @%s (ID: %d). bot.Self.ID початково був 0.", userInfo.UserName, userInfo.ID)
		// Якщо NewBotAPI правильно заповнює bot.Self (навіть якщо ID=0), то цього достатньо.
		// Якщо bot.Self.ID так і залишиться 0, логіка в main.go це залогує.
	} else {
		log.Printf("Бот успішно ініціалізований: ID=%d, UserName='%s'", bot.Self.ID, bot.Self.UserName)
	}
	return bot, nil
}

// HandleUpdates викликає HandleUpdate (яка має бути визначена в handler.go)
func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) {
	log.Println("Розпочато обробку оновлень Telegram...")
	for update := range updates {
		go HandleUpdate(bot, update, srv, cfg)
	}
	log.Println("Зупинено обробку оновлень Telegram (канал закрито).")
}

// SetWebhook: ПОВЕРТАЄМОСЯ ДО ОЧІКУВАННЯ ДВОХ ЗНАЧЕНЬ ВІД NewWebhook...
// Це відповідає тому, як компілятор, схоже, бачить ці функції.
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error {
	if webhookBaseURL == "" || webhookPath == "" {
		log.Println("ПОПЕРЕДЖЕННЯ: WebhookBaseURL або WebhookPath не вказані. Вебхук не буде встановлено.")
		return nil
	}
	log.Printf("Встановлення вебхука: URL=%s%s, CertFile (якщо є)=%s", webhookBaseURL, webhookPath, certFilePath)
	fullWebhookURL := webhookBaseURL + webhookPath
	if !strings.HasPrefix(fullWebhookURL, "https://") && webhookBaseURL != "" { 
		log.Printf("ПОПЕРЕДЖЕННЯ: URL вебхука '%s' не починається з https://.", fullWebhookURL)
	}

	var whCfg tgbotapi.WebhookConfig
	var errWebhookSetup error 

	if certFilePath != "" {
		log.Printf("Спроба встановити вебхук з файлом сертифіката: %s", certFilePath)
		fileBytes := tgbotapi.FilePath(certFilePath)
		whCfg, errWebhookSetup = tgbotapi.NewWebhookWithCert(fullWebhookURL, fileBytes) 
	} else {
		log.Printf("Спроба встановити вебхук без файлу сертифіката.")
		whCfg, errWebhookSetup = tgbotapi.NewWebhook(fullWebhookURL) 
	}

	if errWebhookSetup != nil { 
		log.Printf("ПОМИЛКА конфігурації вебхука NewWebhook...: %v", errWebhookSetup)
		return fmt.Errorf("конфігурація NewWebhook...: %w", errWebhookSetup)
	}

	whCfg.MaxConnections = 40
	resp, err := bot.Request(whCfg) 
	if err != nil {
		log.Printf("ПОМИЛКА встановлення вебхука '%s': %v", fullWebhookURL, err)
		if resp != nil && !resp.Ok { 
			return fmt.Errorf("bot.Request(webhookConfig) failed: %s (code %d)", resp.Description, resp.ErrorCode)
		}
		return fmt.Errorf("bot.Request(webhook setup) failed: %w", err)
	}
	if !resp.Ok { 
		log.Printf("ПОМИЛКА встановлення вебхука: відповідь API не OK. Код: %d, Опис: %s", resp.ErrorCode, resp.Description)
		return fmt.Errorf("встановлення вебхука не вдалося: %s (код %d)", resp.Description, resp.ErrorCode)
	}

	webhookInfo, errInfo := bot.GetWebhookInfo() 
	if errInfo != nil {
		log.Printf("ПОМИЛКА GetWebhookInfo: %v", errInfo)
		return fmt.Errorf("GetWebhookInfo: %w", errInfo)
	}
	if webhookInfo.IsSet() && webhookInfo.URL == fullWebhookURL {
		log.Printf("Вебхук успішно встановлено: URL='%s'", webhookInfo.URL)
		if webhookInfo.LastErrorDate != 0 {
			log.Printf("  Остання помилка вебхука: %s (дата: %s)", webhookInfo.LastErrorMessage, time.Unix(int64(webhookInfo.LastErrorDate), 0).Format(time.RFC3339))
		}
		if webhookInfo.IPAddress != "" {
			log.Printf("  IP-адреса вебхука (як бачить Telegram): %s", webhookInfo.IPAddress)
		}
	} else {
		log.Printf("ПОПЕРЕДЖЕННЯ: Вебхук НЕ встановлено. URL: '%s', Очікувано: '%s'", webhookInfo.URL, fullWebhookURL)
		return fmt.Errorf("вебхук не встановлено: URL '%s' != '%s'", webhookInfo.URL, fullWebhookURL)
	}
	return nil
}

func RemoveWebhook(bot *tgbotapi.BotAPI) error {
	log.Println("Спроба видалення вебхука...")
	_, err := bot.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: true})
	if err != nil { log.Printf("ПОМИЛКА видалення вебхука: %v", err); return fmt.Errorf("видалення вебхука: %w", err) }
	log.Println("Запит на видалення вебхука надіслано."); 
	
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

// ВИДАЛЕНО дублюючі/конфліктуючі визначення, які були у вашій "вчорашній версії" telegram.go:
// - monthNameUkrainian (має бути в report.go)
// - FinancialGoal, SetUserGoal, GetUserGoal, DeleteUserGoal (мають бути в goal.go/closegoal.go)
// - formatDurationToNextFunding (має бути в handler.go)
// Це має виправити помилки "redeclared" та "undefined" для цих сутностей.
