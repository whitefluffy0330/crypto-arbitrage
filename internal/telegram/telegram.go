package telegram

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/config" // Не використовується напряму в цьому файлі
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // Не використовується напряму в цьому файлі
	// gsheets "google.golang.org/api/sheets/v4" // Не використовується напряму в цьому файлі
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
var userStatesMutex = &sync.Mutex{}

func SetUserState(chatID int64, state UserState) {
	userStatesMutex.Lock()
	defer userStatesMutex.Unlock()
	userStates[chatID] = state
	log.Printf("Встановлено стан '%s' для ChatID %d", state, chatID)
}

func GetUserState(chatID int64) UserState {
	userStatesMutex.Lock()
	defer userStatesMutex.Unlock()
	s, ok := userStates[chatID]
	if !ok {
		return StateDefault
	}
	return s
}

var userFundingThresholds = make(map[int64]float64)
var fundingThresholdMutex = &sync.Mutex{}

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
		log.Println("Часову зону Europe/Kyiv завантажено (telegram init).")
	}
	userStates = make(map[int64]UserState)
	userFundingThresholds = make(map[int64]float64)
}

func InitBot(token string) (*tgbotapi.BotAPI, error) {
	log.Println("Спроба ініціалізації бота через tgbotapi.NewBotAPI...")
	if token == "" {
		return nil, fmt.Errorf("токен бота порожній")
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Помилка tgbotapi.NewBotAPI: %v", err)
		return nil, fmt.Errorf("NewBotAPI: %w", err)
	}
	if bot == nil {
		log.Printf("КРИТИЧНА ПОМИЛКА: bot is nil після NewBotAPI")
		return nil, fmt.Errorf("bot is nil after NewBotAPI")
	}

	// Припускаємо, що компілятор бачить bot.Self як структуру User,
	// тому порівняння з nil неможливе і викликає помилку "mismatched types".
	// Просто перевіряємо ID.
	if bot.Self.ID == 0 {
		log.Printf("ПОПЕРЕДЖЕННЯ: bot.Self.ID = 0 після NewBotAPI. UserName: '%s'. Спроба GetMe().", bot.Self.UserName)
		userInfo, errGetMe := bot.GetMe()
		if errGetMe != nil {
			log.Printf("КРИТИЧНА ПОМИЛКА: GetMe() провалився: %v", errGetMe)
			return nil, fmt.Errorf("GetMe() failed: %w", errGetMe)
		}
		if userInfo == nil || userInfo.ID == 0 {
			log.Printf("КРИТИЧНА ПОМИЛКА: GetMe() повернув nil або користувача з ID 0")
			return nil, fmt.Errorf("GetMe() returned nil or zero ID user")
		}
		// Якщо bot.Self - структура, ми не можемо присвоїти userInfo (*User) до неї.
		// Ми можемо тільки оновити поля, якщо це можливо і потрібно.
		// bot.Self = *userInfo // Це викличе помилку, якщо bot.Self - не *User
		// Залишаємо як є, сподіваючись, що NewBotAPI заповнив Self.ID, або GetMe() допоможе.
		log.Printf("Дані бота отримано через GetMe(): ID=%d, UserName='%s'. Початковий bot.Self.ID був 0.", userInfo.ID, userInfo.UserName)
        // Можливо, потрібно оновити поля bot.Self тут, якщо воно структура
        bot.Self.ID = userInfo.ID
        bot.Self.UserName = userInfo.UserName
        bot.Self.FirstName = userInfo.FirstName
        // ... і т.д.
	} else {
		log.Printf("Бот успішно ініціалізований: ID=%d, UserName='%s'", bot.Self.ID, bot.Self.UserName)
	}
	return bot, nil
}

// HandleUpdates тепер визначено тут і викликає processUpdate (який має бути вашим основним обробником)
func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) {
	log.Println("Розпочато обробку оновлень Telegram (з telegram.go)...")
	for update := range updates {
		// Тут викликаємо вашу основну логіку обробки, яка, ймовірно,
		// знаходиться у файлі handler.go і називається HandleUpdate або схожим чином.
		// Я назву її processUpdate для уникнення конфлікту, якщо ви скопіюєте
		// цю функцію в handler.go і назвете її HandleUpdate.
		go processUpdate(bot, update, srv, cfg) // Ця функція processUpdate має бути визначена (наприклад, у handler.go)
	}
	log.Println("Зупинено обробку оновлень Telegram (канал закрито).")
}

// SetWebhook: Очікуємо ДВА значення від NewWebhook...
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error {
	if webhookBaseURL == "" || webhookPath == "" {
		log.Println("ПОПЕРЕДЖЕННЯ: WebhookBaseURL або WebhookPath не вказані.")
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
		fileBytes := tgbotapi.FilePath(certFilePath)
		whCfg, errWebhookSetup = tgbotapi.NewWebhookWithCert(fullWebhookURL, fileBytes) 
	} else {
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
