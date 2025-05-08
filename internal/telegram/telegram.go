package telegram

import (
	"fmt" 
	"log"
	// "net/http" // ВИДАЛЕНО
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	// Імпорт sheets ПОТРІБЕН і ВИКОРИСТОВУЄТЬСЯ
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" 
	gsheets "google.golang.org/api/sheets/v4"
)

var KyivLocation *time.Location
func init() { loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Крит. помилка telegram: %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv завантажено (telegram).") } }

type FinancialGoal struct { Amount float64; Currency string; Days int; OriginalText string; SetDate time.Time }
var ( userGoals = make(map[int64]FinancialGoal); userGoalsMutex sync.RWMutex )
var ( userStates = make(map[int64]string); userStatesMutex sync.RWMutex )
const ( StateDefault = ""; StateAwaitingGoalInput = "awaiting_goal"; StateAwaitingInvestmentInput = "awaiting_investment" )

func SetUserState(chatID int64, state string) { userStatesMutex.Lock(); defer userStatesMutex.Unlock(); if state == StateDefault { delete(userStates, chatID) } else { userStates[chatID] = state }; log.Printf("Встановлено стан '%s' для %d", state, chatID) }
func GetUserState(chatID int64) string { userStatesMutex.RLock(); defer userStatesMutex.RUnlock(); state, exists := userStates[chatID]; if !exists { return StateDefault }; return state }

// --- Функції для роботи з цілями ---
func SetUserGoal(chatID int64, goal FinancialGoal, srv *gsheets.Service, cfg config.Config) error { 
    goalDataForSheet := sheets.FinancialGoalData{ Amount: goal.Amount, Currency: goal.Currency, Days: 0, OriginalText: goal.OriginalText, SetDate: goal.SetDate }; 
    // Викликаємо функцію з пакету sheets
    err := sheets.AddGoalToSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, goalDataForSheet); 
    if err != nil { log.Printf("ПОМИЛКА запису цілі в Sheets для %d: %v", chatID, err); return fmt.Errorf("збереження Sheets: %w", err) }; 
    userGoalsMutex.Lock(); userGoals[chatID] = goal; userGoalsMutex.Unlock(); log.Printf("Ціль збережена для %d", chatID); return nil 
}
func GetUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) (FinancialGoal, bool) { 
    userGoalsMutex.RLock(); goal, exists := userGoals[chatID]; userGoalsMutex.RUnlock(); if exists { return goal, true }; log.Printf("Шукаємо ціль в Sheets для %d", chatID); 
    // Викликаємо функцію з пакету sheets
    sheetGoalData, foundInSheet, err := sheets.GetActiveGoalFromSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID); 
    if err != nil { log.Printf("Помилка завантаження цілі з Sheets: %v", err); return FinancialGoal{}, false }; if foundInSheet { loadedGoal := FinancialGoal{ Amount: sheetGoalData.Amount, Currency: sheetGoalData.Currency, Days: sheetGoalData.Days, OriginalText: sheetGoalData.OriginalText, SetDate: sheetGoalData.SetDate }; userGoalsMutex.Lock(); userGoals[chatID] = loadedGoal; userGoalsMutex.Unlock(); return loadedGoal, true }; return FinancialGoal{}, false 
}
func DeleteUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) error { 
    // Викликаємо функцію з пакету sheets
    err := sheets.UpdateGoalStatusInSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, "Закрита", time.Now().UTC()); 
    if err != nil { if err.Error() == "не знайдено активної цілі для оновлення" { log.Printf("Активну ціль не знайдено в Sheets для %d.", chatID) } else { log.Printf("ПОМИЛКА DeleteUserGoal Sheets для %d: %v", chatID, err); return fmt.Errorf("оновлення статусу Sheets: %w", err) } }; 
    ClearInMemoryUserGoal(chatID); log.Printf("Ціль для %d оброблена для закриття.", chatID); if err != nil && err.Error() != "не знайдено активної цілі для оновлення" { return err }; return nil 
}
func ClearInMemoryUserGoal(chatID int64) { userGoalsMutex.Lock(); defer userGoalsMutex.Unlock(); if _, exists := userGoals[chatID]; exists { delete(userGoals, chatID); log.Printf("Ціль %d видалено з кешу.", chatID) } else { log.Printf("Ціль %d вже відсутня в кеші.", chatID) } }

// --- Основні функції бота ---

// InitBot ПОВЕРНУТО до стандартного виклику tgbotapi.NewBotAPI
func InitBot(token string) (*tgbotapi.BotAPI, error) {
	log.Println("Спроба ініціалізації бота через tgbotapi.NewBotAPI...")
	if token == "" { return nil, fmt.Errorf("токен бота порожній") }
	// Стандартний виклик
	bot, err := tgbotapi.NewBotAPI(token) 
	if err != nil { log.Printf("Помилка NewBotAPI: %v", err); return nil, err }
	return bot, nil 
}

// HandleUpdates приймає cfg config.Config
func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) { log.Println("Розпочато обробку оновлень..."); for update := range updates { HandleUpdate(bot, update, srv, cfg) }; log.Println("Зупинено обробку оновлень.") }
// SetWebhook
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error { /* ... повний код з #149 ... */ return nil }
// RemoveWebhook
func RemoveWebhook(bot *tgbotapi.BotAPI) error { /* ... повний код з #149 ... */ return nil }
