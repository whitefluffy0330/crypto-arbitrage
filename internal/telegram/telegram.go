package telegram

import (
	"fmt" // Додано для fmt.Errorf у GetUserGoal
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Імпортуємо ваш пакет sheets для Sheets.FinancialGoalData та функцій роботи з таблицею
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4"
)

// --- Структура для фінансової цілі ---
type FinancialGoal struct {
	Amount       float64
	Currency     string
	Days         int
	OriginalText string
	SetDate      time.Time // Зберігаємо в UTC, відображаємо в локальному
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

// SetUserGoal зберігає ціль в пам'яті та додає її в Google Sheet.
func SetUserGoal(chatID int64, goal FinancialGoal, srv *gsheets.Service, spreadsheetID string) error {
	goalDataForSheet := sheets.FinancialGoalData{
		Amount:       goal.Amount,
		Currency:     goal.Currency,
		Days:         goal.Days,
		OriginalText: goal.OriginalText,
		SetDate:      goal.SetDate, // Передаємо як є (UTC)
	}

	err := sheets.AddGoalToSheet(srv, spreadsheetID, chatID, goalDataForSheet)
	if err != nil {
		log.Printf("ПОМИЛКА при спробі записати ціль у Google Sheet для ChatID %d: %v", chatID, err)
		return fmt.Errorf("не вдалося зберегти ціль у Google Таблиці: %w", err)
	}

	userGoalsMutex.Lock()
	defer userGoalsMutex.Unlock()
	userGoals[chatID] = goal // Зберігаємо в пам'яті лише після успішного запису в таблицю
	log.Printf("Ціль для чату %d встановлено/оновлено в пам'яті та Google Sheets: %+v", chatID, goal)
	return nil 
}

// GetUserGoal отримує поточну активну ціль користувача (спочатку з пам'яті, потім з Google Sheets).
// Тепер приймає srv та spreadsheetID для можливості завантаження з таблиці.
func GetUserGoal(chatID int64, srv *gsheets.Service, spreadsheetID string) (FinancialGoal, bool) {
	// 1. Перевіряємо кеш в пам'яті
	userGoalsMutex.RLock()
	goal, exists := userGoals[chatID]
	userGoalsMutex.RUnlock()

	if exists {
		log.Printf("Ціль для ChatID %d знайдено в кеші пам'яті.", chatID)
		return goal, true
	}

	// 2. Якщо в пам'яті немає, спробуємо завантажити з Google Sheets
	log.Printf("Ціль для ChatID %d не знайдено в пам'яті, спроба завантаження з Google Sheets...", chatID)
	sheetGoalData, foundInSheet, err := sheets.GetActiveGoalFromSheet(srv, spreadsheetID, chatID)
	if err != nil {
		log.Printf("Помилка завантаження активної цілі з Google Sheets для ChatID %d: %v", chatID, err)
		// Не знайшли або сталася помилка - повертаємо, що цілі немає
		return FinancialGoal{}, false
	}

	if foundInSheet {
		log.Printf("Активну ціль для ChatID %d завантажено з Google Sheets. Зберігаємо в кеш.", chatID)
		// Конвертуємо sheetGoalData (sheets.FinancialGoalData) у FinancialGoal (telegram.FinancialGoal)
		// Припускаємо, що SetDate з таблиці прийшло в UTC (бо ми його парсили як UTC у GetActiveGoalFromSheet)
		loadedGoal := FinancialGoal{
			Amount:       sheetGoalData.Amount,
			Currency:     sheetGoalData.Currency,
			Days:         sheetGoalData.Days,
			OriginalText: sheetGoalData.OriginalText,
			SetDate:      sheetGoalData.SetDate, // Вже має бути UTC
		}

		// Зберігаємо знайдену ціль у кеш пам'яті
		userGoalsMutex.Lock()
		userGoals[chatID] = loadedGoal
		userGoalsMutex.Unlock()

		return loadedGoal, true
	}

	// Якщо не знайдено ні в пам'яті, ні в таблиці
	return FinancialGoal{}, false
}

// DeleteUserGoal видаляє ціль з пам'яті та оновлює її статус у Google Sheet.
func DeleteUserGoal(chatID int64, srv *gsheets.Service, spreadsheetID string) error {
	// Спочатку оновлюємо статус в таблиці
	err := sheets.UpdateGoalStatusInSheet(srv, spreadsheetID, chatID, "Закрита", time.Now().UTC())
	if err != nil {
		// Якщо не вдалося знайти активну ціль в таблиці (вже закрита або немає),
		// функція UpdateGoalStatusInSheet поверне nil (як ми зробили).
		// Якщо була інша помилка (наприклад, API), то повернемо її.
		if err.Error() == "не знайдено активної цілі для оновлення" {
			log.Printf("DeleteUserGoal: Не знайдено активної цілі для закриття в таблиці для ChatID %d.", chatID)
		} else {
			log.Printf("ПОМИЛКА при спробі оновити статус цілі в Google Sheet на 'Закрита' для ChatID %d: %v", chatID, err)
			return fmt.Errorf("не вдалося оновити статус цілі у Google Таблиці: %w", err)
		}
		// Навіть якщо в таблиці не знайшли, спробуємо видалити з пам'яті
	}
	
	// Видаляємо з локального кешу (мапи)
	userGoalsMutex.Lock()
	defer userGoalsMutex.Unlock()
	// Перевіряємо, чи існує в кеші перед видаленням (опціонально)
	if _, exists := userGoals[chatID]; exists {
		delete(userGoals, chatID)
		log.Printf("Ціль для чату %d видалено з пам'яті.", chatID)
	} else {
		log.Printf("Ціль для чату %d вже була відсутня в пам'яті.", chatID)
	}
	
	// Повертаємо nil, якщо оновлення статусу в таблиці пройшло успішно або ціль не була знайдена активною
	if err != nil && err.Error() != "не знайдено активної цілі для оновлення" {
		return err // Повертаємо лише "справжні" помилки API/оновлення
	}
	return nil 
}

// --- Основні функції бота ---
// InitBot, HandleUpdates, SetWebhook, RemoveWebhook залишаються без змін порівняно з останнім оновленням
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
