package telegram

import (
	// "encoding/json" // Не використовувався у вашому оригіналі telegram.go
	"fmt"
	"log"
	"net/http" // Був у вашому оригіналі
	// "os" // Не використовувався у вашому оригіналі telegram.go
	// "strconv" // Не використовувався у вашому оригіналі telegram.go
	"strings"
	"sync" // <--- ДОДАНО, оскільки використовуються м'ютекси
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // Поки закоментуємо, якщо KyivLocation тут
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" // Закоментовано, якщо типи звідси не потрібні напряму в цьому файлі
	gsheets "google.golang.org/api/sheets/v4" // Додано аліас, якщо він потрібен (якщо ні, можна видалити)
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
var userStatesMutex = &sync.Mutex{} // Змінено з userMutex, як було у вас, але з правильним типом

// SetUserState встановлює стан для користувача
func SetUserState(chatID int64, state UserState) {
	userStatesMutex.Lock()
	defer userStatesMutex.Unlock()
	userStates[chatID] = state
	log.Printf("Встановлено стан '%s' для ChatID %d", state, chatID)
}

// GetUserState повертає стан для користувача
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
var fundingThresholdMutex = &sync.Mutex{}

const defaultFundingThreshold = 0.0005 // 0.05%

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

// Timezone
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
	// userGoals ініціалізується там, де визначено
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

	if bot == nil {
		log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: bot є nil після NewBotAPI, хоча помилки не було.")
		return nil, fmt.Errorf("bot is nil after NewBotAPI without an error")
	}

	// Дуже обережна перевірка, щоб уникнути "mismatched types"
	// Якщо bot.Self все ж таки викликає помилку, це може бути глибока проблема з компілятором/бібліотекою
	var botID int64
	var userName string
	if bSelf := bot.Self; bSelf != nil { // <--- ЦЕЙ РЯДОК (АБО ЙОГО ВАРІАЦІЯ) ВИКЛИКАВ ПОМИЛКУ
		botID = bSelf.ID
		userName = bSelf.UserName
	} else {
		// Якщо bot.Self nil, спробуємо GetMe()
		log.Printf("ПОПЕРЕДЖЕННЯ: bot.Self є nil. Спроба викликати GetMe().")
		userInfo, errGetMe := bot.GetMe()
		if errGetMe != nil {
			log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: GetMe() повернув помилку: %v. Можливо, невалідний токен.", errGetMe)
			return nil, fmt.Errorf("GetMe failed after NewBotAPI: %w. Token might be invalid", errGetMe)
		}
		if userInfo.ID == 0 {
			log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: GetMe() повернув userInfo.ID = 0.")
			return nil, fmt.Errorf("GetMe returned userInfo.ID 0, unexpected")
		}
		// bot.Self = userInfo // Не потрібно присвоювати напряму, якщо userInfo - це *User
		botID = userInfo.ID
		userName = userInfo.UserName
		log.Printf("Інформацію про бота отримано через GetMe(): @%s (ID: %d)", userName, botID)
		// Оновимо bot.Self, якщо це потрібно для інших частин коду
		// bot.Self = userInfo // Це може бути проблемою, якщо userInfo не того ж типу, що очікує bot.Self
	}


	if botID == 0 {
		log.Printf("ПОПЕРЕДЖЕННЯ: botID = 0 після ініціалізації. UserName: '%s'. Перевірте токен.", userName)
	} else {
		log.Printf("Бот успішно ініціалізований: ID=%d, UserName='%s'", botID, userName)
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
// ПОВЕРТАЄМОСЯ ДО ЛОГІКИ З ОБРОБКОЮ ДВОХ ЗНАЧЕНЬ ВІД NewWebhook... ЯКЩО КОМПІЛЯТОР НА ЦЬОМУ НАПОЛЯГАЄ
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
	var errWebhookSetup error // <--- ПОВЕРТАЄМО ЗМІННУ ДЛЯ ПОМИЛКИ

	if certFilePath != "" {
		log.Printf("Спроба встановити вебхук з файлом сертифіката: %s", certFilePath)
		fileBytes := tgbotapi.FilePath(certFilePath)
		whCfg, errWebhookSetup = tgbotapi.NewWebhookWithCert(fullWebhookURL, fileBytes) // <--- ОЧІКУЄМО ДВА ЗНАЧЕННЯ
	} else {
		log.Printf("Спроба встановити вебхук без файлу сертифіката.")
		whCfg, errWebhookSetup = tgbotapi.NewWebhook(fullWebhookURL) // <--- ОЧІКУЄМО ДВА ЗНАЧЕННЯ
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

// FinancialGoal, SetUserGoal, GetUserGoal, DeleteUserGoal, ClearInMemoryUserGoal 
// НЕ ВИЗНАЧЕНІ ТУТ, оскільки вони, ймовірно, в інших файлах пакету telegram (напр., goal.go)
// або мають бути визначені тут, якщо це основний файл для них.
// Якщо вони в інших файлах, то помилки "undefined" на них вказують на проблеми з компіляцією
// цих файлів або на те, що вони не включені в збірку.

// monthNameUkrainian та formatDurationToNextFunding ТАКОЖ НЕ ВИЗНАЧЕНІ ТУТ,
// оскільки вони викликали помилку "redeclared". Вони мають бути в одному місці
// (наприклад, у report.go та handler.go відповідно, або у спільному файлі утиліт).
