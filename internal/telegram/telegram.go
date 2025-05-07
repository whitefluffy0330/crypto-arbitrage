package telegram

import (
	"log"
	"sync" // Для sync.RWMutex
	"time" // Додамо для дати встановлення цілі

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	gsheets "google.golang.org/api/sheets/v4"
)

// --- Структура для фінансової цілі ---
type FinancialGoal struct {
	Amount       float64   // Сума цілі
	Currency     string    // Валюта (наприклад, "грн", "USD")
	Days         int       // Кількість днів для досягнення
	OriginalText string    // Початковий текст, введений користувачем
	SetDate      time.Time // Дата встановлення цілі
}

// --- Зберігання даних ---

// userGoals тепер зберігає об'єкти FinancialGoal
var (
	userGoals      = make(map[int64]FinancialGoal) // Змінено тип значення
	userGoalsMutex sync.RWMutex
)

// userStates зберігає поточний стан діалогу для кожного користувача
var (
	userStates      = make(map[int64]string)
	userStatesMutex sync.RWMutex
)

// --- Константи для станів ---
const (
	StateDefault           = ""
	StateAwaitingGoalInput  = "awaiting_goal"
)

// --- Функції для роботи зі станом користувача (з м'ютексом) ---
func SetUserState(chatID int64, state string) {
	userStatesMutex.Lock()
	defer userStatesMutex.Unlock()
	if state == StateDefault {
		delete(userStates, chatID)
	} else {
		userStates[chatID] = state
	}
	log.Printf("Встановлено стан '%s' для чату %d", state, chatID)
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

// --- Функції для роботи з цілями (з м'ютексом) ---
// Ці функції будуть використовуватися HandleGoalInput та іншими

// SetUserGoal зберігає або оновлює ціль для користувача
func SetUserGoal(chatID int64, goal FinancialGoal) {
	userGoalsMutex.Lock()
	defer userGoalsMutex.Unlock()
	userGoals[chatID] = goal
	log.Printf("Ціль для чату %d встановлено/оновлено: %+v", chatID, goal)
}

// GetUserGoal отримує поточну ціль користувача
func GetUserGoal(chatID int64) (FinancialGoal, bool) {
	userGoalsMutex.RLock()
	defer userGoalsMutex.RUnlock()
	goal, exists := userGoals[chatID]
	return goal, exists
}

// DeleteUserGoal видаляє ціль для користувача (може знадобитися для /closegoal)
func DeleteUserGoal(chatID int64) {
	userGoalsMutex.Lock()
	defer userGoalsMutex.Unlock()
	delete(userGoals, chatID)
	log.Printf("Ціль для чату %d видалено", chatID)
}


// --- Основні функції бота ---
func InitBot(token string) (*tgbotapi.BotAPI, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Помилка створення екземпляра бота: %v", err)
		return nil, err
	}
	return bot, nil
}

func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, spreadsheetID string) {
	log.Println("Розпочато обробку оновлень...")
	for update := range updates {
		HandleUpdate(bot, update, srv, spreadsheetID)
	}
	log.Println("Зупинено обробку оновлень (канал закрито).")
}

func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error {
	fullWebhookURL := webhookBaseURL + webhookPath
	log.Printf("Встановлення вебхука на: %s", fullWebhookURL)
	var whCfg tgbotapi.WebhookConfig
	var errWh error
	if certFilePath != "" {
		whCfg, errWh = tgbotapi.NewWebhookWithCert(fullWebhookURL, tgbotapi.FilePath(certFilePath))
	} else {
		whCfg, errWh = tgbotapi.NewWebhook(fullWebhookURL)
	}
	if errWh != nil {
		log.Printf("Помилка створення конфігурації вебхука: %v", errWh)
		return errWh
	}
	_, errReq := bot.Request(whCfg)
	if errReq != nil {
		log.Printf("Помилка встановлення вебхука (bot.Request): %v", errReq)
		return errReq
	}
	info, errInfo := bot.GetWebhookInfo()
	if errInfo != nil {
		log.Printf("Помилка отримання інформації про вебхук: %v", errInfo)
	} else {
		if info.LastErrorDate != 0 {
			log.Printf("Помилка останнього зворотного виклику Telegram (вебхук): %s. URL: %s", info.LastErrorMessage, info.URL)
		} else if info.URL == "" {
			log.Printf("Вебхук оброблено, але URL порожній. Перевірте налаштування.")
		} else {
			log.Printf("Вебхук успішно встановлено. URL: %s", info.URL)
		}
	}
	return nil
}

func RemoveWebhook(bot *tgbotapi.BotAPI) error {
	_, err := bot.Request(tgbotapi.DeleteWebhookConfig{})
	if err != nil {
		log.Printf("Помилка видалення вебхука: %v", err)
		return err
	}
	log.Println("Вебхук успішно видалено.")
	return nil
}
