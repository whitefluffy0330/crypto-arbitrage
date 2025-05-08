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
func init() { /* ... код ініціалізації KyivLocation ... */ 
    loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Крит. помилка: не завантажено 'Europe/Kyiv': %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv завантажено (пакет telegram).") }
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

// InitBot ТИМЧАСОВО створює об'єкт БЕЗ виклику NewBotAPI/getMe
func InitBot(token string) (*tgbotapi.BotAPI, error) {
	log.Println("!!! ДІАГНОСТИКА: Створення об'єкта бота вручну БЕЗ NewBotAPI/getMe !!!")
	if token == "" {
		return nil, fmt.Errorf("токен бота порожній") // Додамо перевірку токена
	}
	// Створюємо об'єкт вручну. bot.Self буде nil.
	bot := &tgbotapi.BotAPI{
		Token: token,
		// Self: nil, // Залишається nil
		Client: tgbotapi.NewClient(token), // Використовуємо стандартний клієнт
		Buffer: 100, // Стандартний розмір буфера
	}
	
	// Оскільки ми не викликали NewBotAPI, помилки API тут не буде.
	// Повертаємо створений бот та nil помилку.
	return bot, nil 
}

// HandleUpdates приймає cfg config.Config
func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) {
	log.Println("Розпочато обробку оновлень...")
	for update := range updates {
		HandleUpdate(bot, update, srv, cfg)
	}
	log.Println("Зупинено обробку оновлень (канал закрито).")
}

// SetWebhook і RemoveWebhook залишаються без змін
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error { /* ... код без змін ... */ return nil }
func RemoveWebhook(bot *tgbotapi.BotAPI) error { /* ... код без змін ... */ return nil }
