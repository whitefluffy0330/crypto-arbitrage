package telegram

import (
	"fmt"
	"log"
	"strings"
	"sync" 
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4" 
)

var KyivLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Printf("Крит. помилка telegram: не вдалося завантажити часову зону 'Europe/Kyiv': %v.", err)
		KyivLocation = time.UTC
	} else {
		KyivLocation = loc
		log.Println("Часову зону Europe/Kyiv завантажено (telegram).")
	}
	userStates = make(map[int64]UserState)
	userGoals = make(map[int64]FinancialGoal)
	userFundingThresholds = make(map[int64]float64)
}

type UserState string
const (
	StateDefault                 UserState = ""
	StateAwaitingGoalInput       UserState = "awaiting_goal_input"
	StateAwaitingInvestmentInput UserState = "awaiting_investment_input"
	StateAwaitingFundingThreshold UserState = "awaiting_funding_threshold"
)
var userStates = make(map[int64]UserState)
var userStatesMutex = &sync.Mutex{} 

func SetUserState(chatID int64, state UserState) {
	userStatesMutex.Lock(); defer userStatesMutex.Unlock()
	userStates[chatID] = state
	log.Printf("Встановлено стан '%s' для ChatID %d", state, chatID)
}
func GetUserState(chatID int64) UserState {
	userStatesMutex.Lock(); defer userStatesMutex.Unlock()
	s, ok := userStates[chatID]; if !ok { return StateDefault }; return s
}

type FinancialGoal struct { Amount float64; Currency string; Days int; OriginalText string; SetDate time.Time }
var ( userGoals = make(map[int64]FinancialGoal); userGoalsMutex sync.RWMutex )

func SetUserGoal(chatID int64, goal FinancialGoal, srv *gsheets.Service, cfg config.Config) error {
	activeGoal, isActive := GetUserGoal(chatID, srv, cfg)
	if isActive {
		log.Printf("Для ChatID %d знайдено активну ціль (%+v) перед встановленням нової. Оновлюємо її статус на 'Перевизначено'.", chatID, activeGoal)
		errUpdateOld := sheets.UpdateGoalStatusInSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, "Перевизначено", time.Now().UTC())
		if errUpdateOld != nil { log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося оновити статус старої цілі для ChatID %d: %v", chatID, errUpdateOld) }
	}
	goalDataForSheet := sheets.FinancialGoalData{ Amount: goal.Amount, Currency: goal.Currency, Days: 0, OriginalText: goal.OriginalText, SetDate: goal.SetDate }
	err := sheets.AddGoalToSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, goalDataForSheet)
	if err != nil { log.Printf("ПОМИЛКА запису нової цілі в Sheets для ChatID %d: %v", chatID, err); return fmt.Errorf("збереження нової цілі: %w", err) }
	userGoalsMutex.Lock(); userGoals[chatID] = goal; userGoalsMutex.Unlock()
	log.Printf("Нову ціль успішно збережено для ChatID %d: %+v", chatID, goal)
	return nil
}
func GetUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) (FinancialGoal, bool) {
	userGoalsMutex.RLock(); goal, exists := userGoals[chatID]; userGoalsMutex.RUnlock()
	if exists { return goal, true }
	sheetGoalData, foundInSheet, err := sheets.GetActiveGoalFromSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID)
	if err != nil { log.Printf("ПОМИЛКА завантаження цілі з Sheets для ChatID %d: %v", chatID, err); return FinancialGoal{}, false }
	if foundInSheet {
		loadedGoal := FinancialGoal{ Amount: sheetGoalData.Amount, Currency: sheetGoalData.Currency, Days: sheetGoalData.Days, OriginalText: sheetGoalData.OriginalText, SetDate: sheetGoalData.SetDate }
		userGoalsMutex.Lock(); userGoals[chatID] = loadedGoal; userGoalsMutex.Unlock()
		log.Printf("Активну ціль для ChatID %d завантажено з Sheets: %+v", chatID, loadedGoal)
		return loadedGoal, true
	}
	return FinancialGoal{}, false
}
func DeleteUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) error {
	err := sheets.UpdateGoalStatusInSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, "Закрита", time.Now().UTC())
	if err != nil {
		if strings.Contains(err.Error(), "не знайдено активної цілі") { ClearInMemoryUserGoal(chatID); return nil }
		ClearInMemoryUserGoal(chatID); return fmt.Errorf("оновлення статусу цілі: %w", err)
	}
	ClearInMemoryUserGoal(chatID); log.Printf("Ціль для ChatID %d закрито.", chatID); return nil
}
func ClearInMemoryUserGoal(chatID int64) {
	userGoalsMutex.Lock(); defer userGoalsMutex.Unlock()
	if _, exists := userGoals[chatID]; exists { delete(userGoals, chatID); log.Printf("Ціль для ChatID %d видалено з кешу.", chatID) }
}

var ( userFundingThresholds = make(map[int64]float64); fundingThresholdMutex = &sync.Mutex{} ) // Змінено на Mutex, як у вашому оригіналі
const defaultFundingThreshold = 0.0005
func SetUserFundingThreshold(chatID int64, threshold float64) {
	fundingThresholdMutex.Lock(); defer fundingThresholdMutex.Unlock()
	userFundingThresholds[chatID] = threshold; log.Printf("Встановлено поріг фандингу %.4f%% для ChatID %d", threshold*100, chatID)
}
func GetUserFundingThreshold(chatID int64) float64 {
	fundingThresholdMutex.Lock(); defer fundingThresholdMutex.Unlock() // Змінено на Lock
	threshold, exists := userFundingThresholds[chatID]
	if !exists { return defaultFundingThreshold }; return threshold
}

// InitBot: повертаємося до логіки, максимально близької до вашого оригіналу,
// припускаючи, що NewBotAPI заповнює bot.Self (типу User), якщо немає помилки.
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
	// Тут ми НЕ перевіряємо bot == nil або bot.Self == nil, щоб уникнути помилки "mismatched types"
	// Це ризиковано, якщо NewBotAPI може повернути nil bot або nil bot.Self без помилки.
	// Ця логіка відповідає вашому оригінальному коду з коміту c230a60.
	if bot.Self.ID == 0 { // Якщо тут паніка, то bot.Self був nil. Якщо "mismatched types" - проблема глибша.
		log.Printf("ПОПЕРЕДЖЕННЯ: bot.Self.ID = 0 після NewBotAPI. UserName: '%s'. Перевірте токен.", bot.Self.UserName)
	} else {
		log.Printf("Бот успішно ініціалізований: ID=%d, UserName='%s'", bot.Self.ID, bot.Self.UserName)
	}
	return bot, nil
}

func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) {
	log.Println("Розпочато обробку оновлень Telegram...")
	for update := range updates {
		go HandleUpdate(bot, update, srv, cfg)
	}
	log.Println("Зупинено обробку оновлень Telegram (канал закрито).")
}

// SetWebhook: ПОВЕРТАЄМОСЯ ДО ОЧІКУВАННЯ ДВОХ ЗНАЧЕНЬ ВІД NewWebhook...
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error {
	if webhookBaseURL == "" || webhookPath == "" {
		log.Println("ПОПЕРЕДЖЕННЯ: WebhookBaseURL або WebhookPath не вказані.")
		return nil
	}
	log.Printf("Встановлення вебхука: URL=%s%s, CertFile (якщо є)=%s", webhookBaseURL, webhookPath, certFilePath)
	fullWebhookURL := webhookBaseURL + webhookPath
	if !strings.HasPrefix(fullWebhookURL, "https://") && webhookBaseURL != "" {
		log.Printf("ПОПЕРЕДЖЕННЯ: URL вебхука '%s' не починається з https://.", fullWebhookURL)
	}

	var whCfg tgbotapi.WebhookConfig
	var errWebhookSetup error 

	if certFilePath != "" {
		fileBytes := tgbotapi.FilePath(certFilePath)
		whCfg, errWebhookSetup = tgbotapi.NewWebhookWithCert(fullWebhookURL, fileBytes) 
	} else {
		whCfg, errWebhookSetup = tgbotapi.NewWebhook(fullWebhookURL) 
	}

	if errWebhookSetup != nil { 
		log.Printf("ПОМИЛКА конфігурації вебхука NewWebhook...: %v", errWebhookSetup)
		return fmt.Errorf("конфігурація NewWebhook...: %w", errWebhookSetup)
	}

	whCfg.MaxConnections = 40 // Як у вашому оригінальному коді
	resp, err := bot.Request(whCfg) 
	if err != nil {
		log.Printf("ПОМИЛКА встановлення вебхука '%s': %v", fullWebhookURL, err)
		if resp != nil && !resp.Ok { 
			return fmt.Errorf("bot.Request(webhookConfig) failed: %s (code %d)", resp.Description, resp.ErrorCode)
		}
		return fmt.Errorf("bot.Request(webhook setup) failed: %w", err)
	}
	if !resp.Ok { 
		log.Printf("ПОМИЛКА встановлення вебхука: відповідь API не OK. Код: %d, Опис: %s", resp.ErrorCode, resp.Description)
		return fmt.Errorf("встановлення вебхука не вдалося: %s (код %d)", resp.Description, resp.ErrorCode)
	}

	webhookInfo, errInfo := bot.GetWebhookInfo() 
	if errInfo != nil {
		log.Printf("ПОМИЛКА GetWebhookInfo: %v", errInfo)
		return fmt.Errorf("GetWebhookInfo: %w", errInfo)
	}
	if webhookInfo.IsSet() && webhookInfo.URL == fullWebhookURL {
		log.Printf("Вебхук успішно встановлено: URL='%s'", webhookInfo.URL)
		if webhookInfo.LastErrorDate != 0 {
			log.Printf("  Остання помилка вебхука: %s (дата: %s)", webhookInfo.LastErrorMessage, time.Unix(int64(webhookInfo.LastErrorDate), 0).Format(time.RFC3339))
		}
		if webhookInfo.IPAddress != "" {
			log.Printf("  IP-адреса вебхука (як бачить Telegram): %s", webhookInfo.IPAddress)
		}
	} else {
		log.Printf("ПОПЕРЕДЖЕННЯ: Вебхук НЕ встановлено. URL: '%s', Очікувано: '%s'", webhookInfo.URL, fullWebhookURL)
		return fmt.Errorf("вебхук не встановлено: URL '%s' != '%s'", webhookInfo.URL, fullWebhookURL)
	}
	return nil
}

func RemoveWebhook(bot *tgbotapi.BotAPI) error {
	log.Println("Спроба видалення вебхука...")
	_, err := bot.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: true})
	if err != nil { log.Printf("ПОМИЛКА видалення вебхука: %v", err); return fmt.Errorf("видалення вебхука: %w", err) }
	log.Println("Запит на видалення вебхука надіслано."); 
	
	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Printf("ПОМИЛКА отримання інформації про вебхук після запиту на видалення: %v", err)
	} else if info.IsSet() && info.URL != "" {
		log.Printf("ПОПЕРЕДЖЕННЯ: Вебхук все ще встановлений на URL: '%s' після запиту на видалення.", info.URL)
	} else {
		log.Println("Вебхук успішно видалено (або не був встановлений).")
	}
	return nil
}
