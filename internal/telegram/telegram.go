package telegram

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // Потрібен для KyivLocation
	"google.golang.org/api/sheets/v4"
)

// UserState представляє поточний стан діалогу користувача
type UserState string

const (
	StateDefault                  UserState = ""
	StateAwaitingGoalInput        UserState = "awaiting_goal_input"
	StateAwaitingInvestmentInput  UserState = "awaiting_investment_input"
	StateAwaitingFundingThreshold UserState = "awaiting_funding_threshold"
)

var userStates = make(map[int64]UserState)
var userMutex = &sync.Mutex{}

// SetUserState встановлює стан для користувача
func SetUserState(chatID int64, state UserState) {
	userMutex.Lock()
	defer userMutex.Unlock()
	userStates[chatID] = state
}

// GetUserState повертає стан для користувача
func GetUserState(chatID int64) UserState {
	userMutex.Lock()
	defer userMutex.Unlock()
	return userStates[chatID]
}

var userFundingThresholds = make(map[int64]float64)
var fundingThresholdMutex = &sync.Mutex{}

const defaultFundingThreshold = 0.0005 // 0.05%

func SetUserFundingThreshold(chatID int64, threshold float64) {
	fundingThresholdMutex.Lock()
	defer fundingThresholdMutex.Unlock()
	userFundingThresholds[chatID] = threshold
}

func GetUserFundingThreshold(chatID int64) float64 {
	fundingThresholdMutex.Lock()
	defer fundingThresholdMutex.Unlock()
	if threshold, ok := userFundingThresholds[chatID]; ok {
		return threshold
	}
	return defaultFundingThreshold
}

// Timezone
var KyivLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Printf("Помилка завантаження часової зони Europe/Kyiv: %v. Використовується UTC.", err)
		KyivLocation = time.UTC
	} else {
		KyivLocation = loc
		log.Println("Часову зону Europe/Kyiv завантажено (telegram).")
	}
	// Ініціалізація userStates та userFundingThresholds, якщо потрібно
	userStates = make(map[int64]UserState)
	userFundingThresholds = make(map[int64]float64)
}

func InitBot(token string) (*tgbotapi.BotAPI, error) {
	log.Println("Спроба ініціалізації бота через tgbotapi.NewBotAPI...")
	if token == "" {
		return nil, fmt.Errorf("токен бота порожній, перевірте змінну середовища TELEGRAM_TOKEN")
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Помилка tgbotapi.NewBotAPI: %v", err)
		return nil, fmt.Errorf("не вдалося створити BotAPI: %w", err)
	}

	if bot == nil {
		log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: bot є nil після NewBotAPI, хоча помилки не було.")
		return nil, fmt.Errorf("bot is nil after NewBotAPI without an error, unexpected situation")
	}

	// Перевірка bot.Self.ID замість порівняння структури з nil
	if bot.Self.ID == 0 {
		// Спробуємо отримати інформацію про бота ще раз
		userInfo, errGetMe := bot.GetMe()
		if errGetMe != nil {
			log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: bot.Self.ID = 0 І GetMe() повернув помилку: %v. Можливо, невалідний токен.", errGetMe)
			return nil, fmt.Errorf("bot.Self.ID is 0 and GetMe failed: %w. Token might be invalid", errGetMe)
		}
		if userInfo.ID == 0 {
			log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: bot.Self.ID = 0 І GetMe() повернув userInfo.ID = 0. Непередбачена ситуація.")
			return nil, fmt.Errorf("bot.Self.ID is 0 and GetMe returned userInfo.ID 0, unexpected")
		}
		bot.Self = *userInfo // Оновлюємо інформацію про бота
		log.Printf("Інформацію про бота отримано повторним запитом: @%s (ID: %d)", bot.Self.UserName, bot.Self.ID)
	} else {
		log.Printf("Бот успішно ініціалізований: ID=%d, UserName='%s'", bot.Self.ID, bot.Self.UserName)
	}
	return bot, nil
}

func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) {
	log.Println("Розпочато обробку оновлень Telegram...")
	for update := range updates {
		// Обробляємо кожне оновлення в окремій горутині, щоб не блокувати отримання наступних
		go HandleUpdate(bot, update, srv, cfg)
	}
	log.Println("Зупинено обробку оновлень Telegram (канал закрито).")
}

// CreateWebhookConfig створює конфігурацію вебхука.
// Повертає WebhookConfig та помилку, якщо є.
func CreateWebhookConfig(webhookURL string, certFilePath string) (tgbotapi.WebhookConfig, error) {
	var whCfg tgbotapi.WebhookConfig
	var err error

	if certFilePath != "" {
		log.Printf("Спроба створити конфігурацію вебхука з файлом сертифіката: %s", certFilePath)
		// tgbotapi.NewWebhookWithCert повертає (WebhookConfig, error)
		whCfg, err = tgbotapi.NewWebhookWithCert(webhookURL, tgbotapi.FilePath(certFilePath))
		if err != nil {
			return tgbotapi.WebhookConfig{}, fmt.Errorf("створення NewWebhookWithCert: %w", err)
		}
	} else {
		log.Println("Спроба створити конфігурацію вебхука без файлу сертифіката (Nginx має обробляти SSL).")
		// tgbotapi.NewWebhook повертає (WebhookConfig, error)
		whCfg, err = tgbotapi.NewWebhook(webhookURL)
		if err != nil {
			return tgbotapi.WebhookConfig{}, fmt.Errorf("створення NewWebhook: %w", err)
		}
	}
	// Встановлюємо додаткові параметри, якщо потрібно
	// whCfg.MaxConnections = 40 // Наприклад
	return whCfg, nil
}

// SetWebhook встановлює вебхук для бота.
func SetWebhook(bot *tgbotapi.BotAPI, webhookConfig tgbotapi.WebhookConfig) error {
	log.Printf("Встановлення вебхука: URL=%s, HasCustomCert: %t", webhookConfig.URL, webhookConfig.Certificate != nil)

	resp, err := bot.Request(webhookConfig)
	if err != nil {
		log.Printf("ПОМИЛКА bot.Request(webhookConfig) при встановленні вебхука: %v", err)
		// Додаємо деталі відповіді, якщо вони є, для кращого розуміння помилки
		if resp != nil && !resp.Ok {
			return fmt.Errorf("bot.Request(webhookConfig) failed: %s (code %d)", resp.Description, resp.ErrorCode)
		}
		return fmt.Errorf("bot.Request(webhookConfig) failed: %w", err)
	}

	if !resp.Ok {
		log.Printf("ПОМИЛКА встановлення вебхука: відповідь API не OK. Код: %d, Опис: %s", resp.ErrorCode, resp.Description)
		return fmt.Errorf("встановлення вебхука не вдалося: %s (код %d)", resp.Description, resp.ErrorCode)
	}

	webhookInfo, errInfo := bot.GetWebhookInfo()
	if errInfo != nil {
		log.Printf("ПОМИЛКА отримання інформації про вебхук після встановлення: %v", errInfo)
		return fmt.Errorf("bot.GetWebhookInfo failed after setup: %w", errInfo)
	}

	if webhookInfo.IsSet() && webhookInfo.URL == webhookConfig.URL {
		log.Printf("Вебхук успішно встановлено та перевірено: URL=%s, PendingUpdates=%d", webhookInfo.URL, webhookInfo.PendingUpdateCount)
		if webhookInfo.LastErrorDate != 0 {
			log.Printf("ПОПЕРЕДЖЕННЯ: Є остання помилка вебхука: %s (дата: %s)", webhookInfo.LastErrorMessage, time.Unix(int64(webhookInfo.LastErrorDate), 0).In(KyivLocation).Format(time.RFC3339))
		}
	} else {
		errMsg := fmt.Sprintf("не вдалося перевірити встановлення вебхука. URL очікувався: %s, отримано: %s. IsSet: %t", webhookConfig.URL, webhookInfo.URL, webhookInfo.IsSet())
		log.Println(errMsg)
		return fmt.Errorf(errMsg)
	}
	return nil
}

func monthNameUkrainian(month time.Month) string {
	// ... (код цієї функції без змін з відповіді #323) ...
	switch month {
	case time.January:
		return "Січень"
	case time.February:
		return "Лютий"
	case time.March:
		return "Березень"
	case time.April:
		return "Квітень"
	case time.May:
		return "Травень"
	case time.June:
		return "Червень"
	case time.July:
		return "Липень"
	case time.August:
		return "Серпень"
	case time.September:
		return "Вересень"
	case time.October:
		return "Жовтень"
	case time.November:
		return "Листопад"
	case time.December:
		return "Грудень"
	default:
		return ""
	}
}

// State management
var userMonthGoal = make(map[int64]goal.Goal)
var userMonthGoalMutex = &sync.Mutex{}

func SetUserGoal(chatID int64, g goal.Goal) {
	userMonthGoalMutex.Lock()
	defer userMonthGoalMutex.Unlock()
	userMonthGoal[chatID] = g
}

func GetUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) (goal.Goal, bool) {
	userMonthGoalMutex.Lock()
	g, exists := userMonthGoal[chatID]
	userMonthGoalMutex.Unlock()

	if !exists || g.Status == "Закрита" { // Якщо немає в кеші або закрита, спробуємо завантажити з Sheets
		log.Printf("Активна ціль для ChatID %d не знайдена в кеші. Спроба завантажити з Google Sheets (аркуш: %s).", chatID, cfg.SheetNameUserGoals)
		activeGoal, err := sheets.GetActiveGoal(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID)
		if err != nil {
			log.Printf("Помилка завантаження активної цілі з Sheets для ChatID %d: %v", chatID, err)
			return goal.Goal{}, false
		}
		if activeGoal.Amount > 0 { // Перевіряємо, чи дійсно щось знайдено
			SetUserGoal(chatID, activeGoal) // Оновлюємо кеш
			return activeGoal, true
		}
		log.Printf("Активну ціль для ChatID %d не знайдено ні в кеші, ні в Sheets.", chatID)
		return goal.Goal{}, false
	}
	return g, true
}

func DeleteUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) error {
	userMonthGoalMutex.Lock()
	defer userMonthGoalMutex.Unlock()
	
	g, exists := userMonthGoal[chatID]
	if exists && g.Status == "Активна" { // Закриваємо тільки активну ціль з кешу
		delete(userMonthGoal, chatID) // Видаляємо з кешу
	}
	// Незалежно від кешу, намагаємося закрити в Sheets
	return sheets.CloseActiveGoal(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID)
}

func formatDurationToNextFunding(d time.Duration) string {
	isPast := false
	if d < 0 {
		d = -d
		isPast = true
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours == 0 && minutes == 0 {
		seconds := int(d.Seconds()) % 60
		if isPast {
			return "0с (минув)"
		}
		return fmt.Sprintf("%dс", seconds)
	}

	if isPast {
		return fmt.Sprintf("-%dг %dхв (минув)", hours, minutes)
	}
	return fmt.Sprintf("%dг %dхв", hours, minutes)
}
