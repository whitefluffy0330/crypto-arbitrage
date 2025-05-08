package telegram

import (
	"fmt" // ПОТРІБЕН для fmt.Errorf
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	// ПОТРІБЕН імпорт sheets
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" 
	gsheets "google.golang.org/api/sheets/v4"
)

var KyivLocation *time.Location 
func init() { /* ... код ініціалізації KyivLocation ... */ } 

type FinancialGoal struct { Amount float64; Currency string; Days int; OriginalText string; SetDate time.Time }
var ( userGoals = make(map[int64]FinancialGoal); userGoalsMutex sync.RWMutex )
var ( userStates = make(map[int64]string); userStatesMutex sync.RWMutex )
const ( StateDefault = ""; StateAwaitingGoalInput = "awaiting_goal"; StateAwaitingInvestmentInput = "awaiting_investment" ) // Включаючи новий стан

func SetUserState(chatID int64, state string) { /* ... код без змін з #77 ... */ }
func GetUserState(chatID int64) string { /* ... код без змін з #77 ... */ return "" }

// SetUserGoal ВИКОРИСТОВУЄ sheets.AddGoalToSheet та fmt.Errorf
func SetUserGoal(chatID int64, goal FinancialGoal, srv *gsheets.Service, cfg config.Config) error { 
	goalDataForSheet := sheets.FinancialGoalData{ Amount: goal.Amount, Currency: goal.Currency, Days: 0 /*Запис 0*/, OriginalText: goal.OriginalText, SetDate: goal.SetDate, }; 
	err := sheets.AddGoalToSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, goalDataForSheet); // ВИКОРИСТАННЯ sheets
	if err != nil { return fmt.Errorf("збереження цілі в Sheets: %w", err) }; // ВИКОРИСТАННЯ fmt
	userGoalsMutex.Lock(); userGoals[chatID] = goal; userGoalsMutex.Unlock(); log.Printf("Ціль збережена для %d", chatID); return nil 
}
// GetUserGoal ВИКОРИСТОВУЄ sheets.GetActiveGoalFromSheet
func GetUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) (FinancialGoal, bool) { 
	userGoalsMutex.RLock(); goal, exists := userGoals[chatID]; userGoalsMutex.RUnlock(); if exists { return goal, true }; log.Printf("Шукаємо ціль в Sheets для %d", chatID); 
	sheetGoalData, foundInSheet, err := sheets.GetActiveGoalFromSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID); // ВИКОРИСТАННЯ sheets
	if err != nil { log.Printf("Помилка завантаження цілі з Sheets: %v", err); return FinancialGoal{}, false }; if foundInSheet { loadedGoal := FinancialGoal{ Amount: sheetGoalData.Amount, Currency: sheetGoalData.Currency, Days: sheetGoalData.Days, OriginalText: sheetGoalData.OriginalText, SetDate: sheetGoalData.SetDate, }; userGoalsMutex.Lock(); userGoals[chatID] = loadedGoal; userGoalsMutex.Unlock(); return loadedGoal, true }; return FinancialGoal{}, false 
}
// DeleteUserGoal ВИКОРИСТОВУЄ sheets.UpdateGoalStatusInSheet та fmt.Errorf
func DeleteUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) error { 
	err := sheets.UpdateGoalStatusInSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, "Закрита", time.Now().UTC()); // ВИКОРИСТАННЯ sheets
	if err != nil { if err.Error() == "не знайдено активної цілі для оновлення" { log.Printf("Активну ціль не знайдено в Sheets для %d.", chatID) } else { log.Printf("ПОМИЛКА DeleteUserGoal при оновленні Sheets для %d: %v", chatID, err); return fmt.Errorf("оновлення статусу в Sheets: %w", err) } }; // ВИКОРИСТАННЯ fmt
	ClearInMemoryUserGoal(chatID); log.Printf("Ціль для %d оброблена для закриття.", chatID); if err != nil && err.Error() != "не знайдено активної цілі для оновлення" { return err }; return nil 
}
func ClearInMemoryUserGoal(chatID int64) { /* ... код без змін ... */ }
func InitBot(token string) (*tgbotapi.BotAPI, error) { /* ... код без змін ... */ return nil, nil }
func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) { /* ... код без змін ... */ }
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error { /* ... код без змін (з fmt.Errorf) ... */ return nil }
func RemoveWebhook(bot *tgbotapi.BotAPI) error { /* ... код без змін ... */ return nil }
