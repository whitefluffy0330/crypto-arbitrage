package telegram

import (
	"fmt" 
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4"
)

var KyivLocation *time.Location

func init() { // ... (код ініціалізації KyivLocation без змін) ... 
	loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Крит. помилка: не завантажено 'Europe/Kyiv': %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv завантажено (пакет telegram).") }
}

type FinancialGoal struct { // ... (код структури без змін) ...
	Amount       float64; Currency     string; Days         int; OriginalText string; SetDate      time.Time
}

var ( userGoals = make(map[int64]FinancialGoal); userGoalsMutex sync.RWMutex )
var ( userStates = make(map[int64]string); userStatesMutex sync.RWMutex )

// --- Константи для станів ---
const (
	StateDefault           = ""                // Звичайний стан
	StateAwaitingGoalInput  = "awaiting_goal"     // Очікуємо введення цілі
	// ДОДАНО НОВИЙ СТАН:
	StateAwaitingInvestmentInput = "awaiting_investment" // Очікуємо введення інвестиції 
)

// --- Функції для роботи зі станом користувача (без змін) ---
func SetUserState(chatID int64, state string) { /* ... код без змін ... */ 
	userStatesMutex.Lock(); defer userStatesMutex.Unlock(); if state == StateDefault { delete(userStates, chatID) } else { userStates[chatID] = state }; log.Printf("Встановлено стан '%s' для чату %d", state, chatID)
}
func GetUserState(chatID int64) string { /* ... код без змін ... */ 
	userStatesMutex.RLock(); defer userStatesMutex.RUnlock(); state, exists := userStates[chatID]; if !exists { return StateDefault }; return state
}

// --- Функції для роботи з цілями (без змін) ---
func SetUserGoal(chatID int64, goal FinancialGoal, srv *gsheets.Service, cfg config.Config) error { /* ... код без змін ... */ return nil }
func GetUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) (FinancialGoal, bool) { /* ... код без змін ... */ return FinancialGoal{}, false }
func DeleteUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) error { /* ... код без змін ... */ return nil }
func ClearInMemoryUserGoal(chatID int64) { /* ... код без змін ... */ }

// --- Основні функції бота (без змін) ---
func InitBot(token string) (*tgbotapi.BotAPI, error) { /* ... код без змін ... */ return nil, nil }
func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) { /* ... код без змін ... */ }
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error { /* ... код без змін ... */ return nil }
func RemoveWebhook(bot *tgbotapi.BotAPI) error { /* ... код без змін ... */ return nil }
