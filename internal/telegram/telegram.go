package telegram

import (
	"fmt" 
	"log"
	"net/http" // Залишаємо, якщо SetWebhook/RemoveWebhook його потребують (залежить від бібліотеки) - ні, він не потрібен тут
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4"
)

var KyivLocation *time.Location
func init() { /* ... код ініціалізації KyivLocation ... */ 
    loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Крит. помилка telegram: %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv завантажено (telegram).") }
}

type FinancialGoal struct { Amount float64; Currency string; Days int; OriginalText string; SetDate time.Time }
var ( userGoals = make(map[int64]FinancialGoal); userGoalsMutex sync.RWMutex )
var ( userStates = make(map[int64]string); userStatesMutex sync.RWMutex )
const ( StateDefault = ""; StateAwaitingGoalInput = "awaiting_goal"; StateAwaitingInvestmentInput = "awaiting_investment" )

func SetUserState(chatID int64, state string) { /* ... код без змін ... */ }
func GetUserState(chatID int64) string { /* ... код без змін ... */ return "" }
func SetUserGoal(chatID int64, goal FinancialGoal, srv *gsheets.Service, cfg config.Config) error { /* ... код без змін ... */ return nil }
func GetUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) (FinancialGoal, bool) { /* ... код без змін ... */ return FinancialGoal{}, false }
func DeleteUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) error { /* ... код без змін ... */ return nil }
func ClearInMemoryUserGoal(chatID int64) { /* ... код без змін ... */ }

// --- Основні функції бота ---

// InitBot ПОВЕРТАЄМО до стандартного виклику tgbotapi.NewBotAPI
func InitBot(token string) (*tgbotapi.BotAPI, error) {
	log.Println("Спроба ініціалізації бота через tgbotapi.NewBotAPI...") // Змінено лог
	if token == "" {
		return nil, fmt.Errorf("токен бота порожній") 
	}
	// Стандартний виклик, який включає getMe
	bot, err := tgbotapi.NewBotAPI(token) 
	if err != nil {
		log.Printf("Помилка створення екземпляра бота через NewBotAPI: %v", err) 
		return nil, err // Повертаємо помилку, якщо вона є
	}
	// Якщо помилки немає, повертаємо bot та nil error
	return bot, nil 
}

func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) { /* ... код без змін ... */ }
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error { /* ... код без змін ... */ return nil }
func RemoveWebhook(bot *tgbotapi.BotAPI) error { /* ... код без змін ... */ return nil }
