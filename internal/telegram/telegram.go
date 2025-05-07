package telegram

import (
	"fmt"
	"log"
	"sync"
	"time" // Потрібен для time.Location та ініціалізації

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4"
)

// --- Часова зона ---
var kyivLocation *time.Location // Змінна для часової зони Києва

func init() {
	// Ініціалізуємо часову зону один раз при завантаженні пакета telegram
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Printf("Критична помилка в пакеті telegram: не вдалося завантажити часову зону Europe/Kyiv: %v. Буде використано UTC.", err)
		kyivLocation = time.UTC // Використовуємо UTC як запасний варіант
	} else {
		kyivLocation = loc
		log.Println("Часову зону Europe/Kyiv успішно завантажено (пакет telegram).")
	}
}

// --- Структура для фінансової цілі ---
type FinancialGoal struct {
	Amount       float64
	Currency     string
	Days         int
	OriginalText string
	SetDate      time.Time 
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
func SetUserGoal(chatID int64, goal FinancialGoal, srv *gsheets.Service, spreadsheetID string) error {
	goalDataForSheet := sheets.FinancialGoalData{
		Amount:       goal.Amount,
		Currency:     goal.Currency,
		Days:         goal.Days,
		OriginalText: goal.OriginalText,
		SetDate:      goal.SetDate, // Передаємо UTC
	}

	err := sheets.AddGoalToSheet(srv, spreadsheetID, chatID, goalDataForSheet)
	if err != nil {
		log.Printf("ПОМИЛКА при спробі записати ціль у Google Sheet для ChatID %d: %v", chatID, err)
		return fmt.Errorf("не вдалося зберегти ціль у Google Таблиці: %w", err)
	}

	userGoalsMutex.Lock()
	defer userGoalsMutex.Unlock()
	userGoals[chatID] = goal 
	log.Printf("Ціль для чату %d встановлено/оновлено в пам'яті та Google Sheets: %+v", chatID, goal)
	return nil 
}

func GetUserGoal(chatID int64, srv *gsheets.Service, spreadsheetID string) (FinancialGoal, bool) {
	userGoalsMutex.RLock()
	goal, exists := userGoals[chatID]
	userGoalsMutex.RUnlock()

	if exists {
		log.Printf("Ціль для ChatID %d знайдено в кеші пам'яті.", chatID)
		return goal, true
	}

	log.Printf("Ціль для ChatID %d не знайдено в пам'яті, спроба завантаження з Google Sheets...", chatID)
	sheetGoalData, foundInSheet, err := sheets.GetActiveGoalFromSheet(srv, spreadsheetID, chatID)
	if err != nil {
		log.Printf("Помилка завантаження активної цілі з Google Sheets для ChatID %d: %v", chatID, err)
		return FinancialGoal{}, false
	}

	if foundInSheet {
		log.Printf("Активну ціль для ChatID %d завантажено з Google Sheets. Зберігаємо в кеш.", chatID)
		loadedGoal := FinancialGoal{
			Amount:       sheetGoalData.Amount,
			Currency:     sheetGoalData.Currency,
			Days:         sheetGoalData.Days,
			OriginalText: sheetGoalData.OriginalText,
			SetDate:      sheetGoalData.SetDate, // Вже має бути UTC з GetActiveGoalFromSheet
		}

		userGoalsMutex.Lock()
		userGoals[chatID] = loadedGoal
		userGoalsMutex.Unlock()

		return loadedGoal, true
	}

	return FinancialGoal{}, false
}

func DeleteUserGoal(chatID int64, srv *gsheets.Service, spreadsheetID string) error {
	// Використовуємо time.Now().UTC() для дати закриття
	err := sheets.UpdateGoalStatusInSheet(srv, spreadsheetID, chatID, "Закрита", time.Now().UTC())
	if err != nil {
		if err.Error() == "не знайдено активної цілі для оновлення" {
			log.Printf("DeleteUserGoal: Не знайдено активної цілі для закриття в таблиці для ChatID %d.", chatID)
		} else {
			log.Printf("ПОМИЛКА при спробі оновити статус цілі в Google Sheet на 'Закрита' для ChatID %d: %v", chatID, err)
			return fmt.Errorf("не вдалося оновити статус цілі у Google Таблиці: %w", err)
		}
	}
	
	userGoalsMutex.Lock()
	defer userGoalsMutex.Unlock()
	if _, exists := userGoals[chatID]; exists {
		delete(userGoals, chatID)
		log.Printf("Ціль для чату %d видалено з пам'яті.", chatID)
	} else {
		log.Printf("Ціль для чату %d вже була відсутня в пам'яті.", chatID)
	}
	
	if err != nil && err.Error() != "не знайдено активної цілі для оновлення" {
		return err 
	}
	return nil 
}

// --- Основні функції бота ---
// InitBot, HandleUpdates, SetWebhook, RemoveWebhook залишаються без змін
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
