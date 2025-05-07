package telegram

import (
	"fmt"
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4"
)

var kyivLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Printf("Критична помилка в пакеті telegram: не вдалося завантажити часову зону Europe/Kyiv: %v. Буде використано UTC.", err)
		kyivLocation = time.UTC
	} else {
		kyivLocation = loc
		log.Println("Часову зону Europe/Kyiv успішно завантажено (пакет telegram).")
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
	StateDefault          = ""
	StateAwaitingGoalInput = "awaiting_goal"
)

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

func SetUserGoal(chatID int64, goal FinancialGoal, srv *gsheets.Service, spreadsheetID string) error {
	goalDataForSheet := sheets.FinancialGoalData{
		Amount:       goal.Amount,
		Currency:     goal.Currency,
		Days:         goal.Days,
		OriginalText: goal.OriginalText,
		SetDate:      goal.SetDate,
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
			SetDate:      sheetGoalData.SetDate,
		}
		userGoalsMutex.Lock()
		userGoals[chatID] = loadedGoal
		userGoalsMutex.Unlock()
		return loadedGoal, true
	}
	return FinancialGoal{}, false
}

// DeleteUserGoal тепер викликається з підпакету goal (через HandleCallback)
// Вона відповідає за оновлення статусу в Google Sheet ТА видалення з кешу.
// Якщо виникає помилка в Sheets, вона її повертає.
func DeleteUserGoal(chatID int64, srv *gsheets.Service, spreadsheetID string) error {
	err := sheets.UpdateGoalStatusInSheet(srv, spreadsheetID, chatID, "Закрита", time.Now().UTC())
	if err != nil {
		// UpdateGoalStatusInSheet повертає nil, якщо ціль не знайдено активною,
		// тому перевіряємо на конкретну помилку, перш ніж її повертати.
		if err.Error() != "не знайдено активної цілі для оновлення" {
			log.Printf("ПОМИЛКА в DeleteUserGoal при оновленні статусу в Google Sheet для ChatID %d: %v", chatID, err)
			return fmt.Errorf("не вдалося оновити статус цілі у Google Таблиці: %w", err)
		}
		// Якщо активної цілі не було знайдено в таблиці, помилки немає, але з кешу все одно треба видалити
		log.Printf("DeleteUserGoal: Активну ціль для ChatID %d не знайдено в таблиці (можливо, вже закрита).", chatID)
	}
	
	// Незалежно від результату оновлення в таблиці (якщо це не помилка API),
	// намагаємося видалити з кешу, якщо вона там є.
	ClearInMemoryUserGoal(chatID) // Викликаємо нову функцію для очищення кешу
	
	if err != nil && err.Error() != "не знайдено активної цілі для оновлення" {
		return err
	}
	log.Printf("Ціль для ChatID %d успішно оброблена для закриття (статус в Google Sheets оновлено, з кешу видалено).", chatID)
	return nil
}

// ClearInMemoryUserGoal видаляє ціль користувача з кешу пам'яті.
// Ця функція експортована (починається з великої літери), щоб її міг викликати підпакет goal.
func ClearInMemoryUserGoal(chatID int64) {
	userGoalsMutex.Lock()
	defer userGoalsMutex.Unlock()
	if _, exists := userGoals[chatID]; exists {
		delete(userGoals, chatID)
		log.Printf("Ціль для чату %d видалено з кешу пам'яті.", chatID)
	} else {
		log.Printf("Ціль для чату %d вже була відсутня в кеші пам'яті (ClearInMemoryUserGoal).", chatID)
	}
}

// InitBot, HandleUpdates, SetWebhook, RemoveWebhook залишаються без змін
func InitBot(token string) (*tgbotapi.BotAPI, error) { // ... (код без змін)
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Помилка створення екземпляра бота: %v", err)
		return nil, err
	}
	return bot, nil
}

func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, spreadsheetID string) { // ... (код без змін)
	log.Println("Розпочато обробку оновлень...")
	for update := range updates {
		HandleUpdate(bot, update, srv, spreadsheetID)
	}
	log.Println("Зупинено обробку оновлень (канал закрито).")
}

func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error { // ... (код без змін)
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

func RemoveWebhook(bot *tgbotapi.BotAPI) error { // ... (код без змін)
	_, err := bot.Request(tgbotapi.DeleteWebhookConfig{})
	if err != nil {
		log.Printf("Помилка видалення вебхука: %v", err)
		return err
	}
	log.Println("Вебхук успішно видалено.")
	return nil
}
