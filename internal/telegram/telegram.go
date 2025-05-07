package telegram

import (
	"fmt" // Додано для fmt.Errorf
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Імпортуємо ваш пакет sheets для Sheets.FinancialGoalData та функцій роботи з таблицею
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4"
)

// --- Структура для фінансової цілі (залишається тут для внутрішнього використання ботом) ---
type FinancialGoal struct {
	Amount       float64
	Currency     string
	Days         int
	OriginalText string
	SetDate      time.Time
	// Можна додати ChatID сюди, якщо FinancialGoal буде передаватися як єдина структура
}

// --- Зберігання даних (в пам'яті як кеш) ---
var (
	userGoals      = make(map[int64]FinancialGoal)
	userGoalsMutex sync.RWMutex
)

var (
	userStates      = make(map[int64]string)
	userStatesMutex sync.RWMutex
)

// --- Константи для станів ---
const (
	StateDefault          = ""
	StateAwaitingGoalInput = "awaiting_goal"
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

// --- Функції для роботи з цілями (з м'ютексом ТА інтеграцією з Google Sheets) ---

// SetUserGoal зберігає ціль в пам'яті та намагається додати її в Google Sheet.
// Тепер повертає помилку, якщо не вдалося записати в таблицю.
func SetUserGoal(chatID int64, goal FinancialGoal, srv *gsheets.Service, spreadsheetID string) error {
	// Створюємо дані для передачі в пакет sheets
	goalDataForSheet := sheets.FinancialGoalData{
		Amount:       goal.Amount,
		Currency:     goal.Currency,
		Days:         goal.Days,
		OriginalText: goal.OriginalText,
		SetDate:      goal.SetDate,
	}

	// Намагаємося додати ціль у Google Sheet
	err := sheets.AddGoalToSheet(srv, spreadsheetID, chatID, goalDataForSheet)
	if err != nil {
		log.Printf("ПОМИЛКА при спробі записати ціль у Google Sheet для ChatID %d: %v", chatID, err)
		// Вирішуємо, чи встановлювати ціль в пам'яті, якщо не вдалося записати в таблицю.
		// Поки що, для простоти, повернемо помилку і не будемо зберігати в пам'яті,
		// щоб забезпечити консистентність. Або можна зберігати в пам'яті, але повідомити користувача.
		return fmt.Errorf("не вдалося зберегти ціль у Google Таблиці: %w", err)
	}

	// Якщо запис у таблицю успішний, зберігаємо також у локальний кеш (мапу)
	userGoalsMutex.Lock()
	defer userGoalsMutex.Unlock()
	userGoals[chatID] = goal
	log.Printf("Ціль для чату %d встановлено/оновлено в пам'яті та Google Sheets: %+v", chatID, goal)
	return nil // Успіх
}

// GetUserGoal отримує поточну активну ціль користувача з пам'яті.
// TODO: У майбутньому можна додати логіку завантаження активної цілі з Google Sheets при старті бота
// або якщо цілі немає в пам'яті.
func GetUserGoal(chatID int64) (FinancialGoal, bool) {
	userGoalsMutex.RLock()
	defer userGoalsMutex.RUnlock()
	goal, exists := userGoals[chatID]
	return goal, exists
}

// DeleteUserGoal видаляє ціль з пам'яті та намагається оновити її статус у Google Sheet.
// Тепер повертає помилку, якщо не вдалося оновити статус в таблиці.
func DeleteUserGoal(chatID int64, srv *gsheets.Service, spreadsheetID string) error {
	// Намагаємося оновити статус цілі в Google Sheet
	err := sheets.UpdateGoalStatusInSheet(srv, spreadsheetID, chatID, "Закрита", time.Now().UTC())
	if err != nil {
		log.Printf("ПОМИЛКА при спробі оновити статус цілі в Google Sheet на 'Закрита' для ChatID %d: %v", chatID, err)
		// Повертаємо помилку. Вирішіть, чи видаляти з пам'яті, якщо в таблиці не оновилося.
		// Поки що, для консистентності, не будемо видаляти з пам'яті, якщо не оновили в таблиці.
		return fmt.Errorf("не вдалося оновити статус цілі у Google Таблиці: %w", err)
	}
	
	// Якщо оновлення статусу в таблиці успішне, видаляємо з локального кешу (мапи)
	userGoalsMutex.Lock()
	defer userGoalsMutex.Unlock()
	delete(userGoals, chatID)
	log.Printf("Ціль для чату %d видалено з пам'яті та оновлено статус у Google Sheets.", chatID)
	return nil // Успіх
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
