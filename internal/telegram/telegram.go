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
var userStatesMutex sync.RWMutex

func SetUserState(chatID int64, state UserState) {
	userStatesMutex.Lock(); defer userStatesMutex.Unlock()
	if state == StateDefault { delete(userStates, chatID) } else { userStates[chatID] = state }
	log.Printf("Встановлено стан '%s' для ChatID %d", string(state), chatID)
}
func GetUserState(chatID int64) UserState {
	userStatesMutex.RLock(); defer userStatesMutex.RUnlock()
	state, exists := userStates[chatID]; if !exists { return StateDefault }; return state
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

var ( userFundingThresholds = make(map[int64]float64); userFundingThresholdsMutex sync.RWMutex )
const defaultFundingThreshold = 0.0005
func SetUserFundingThreshold(chatID int64, threshold float64) {
	userFundingThresholdsMutex.Lock(); defer userFundingThresholdsMutex.Unlock()
	userFundingThresholds[chatID] = threshold; log.Printf("Встановлено поріг фандингу %.4f%% для ChatID %d", threshold*100, chatID)
}
func GetUserFundingThreshold(chatID int64) float64 {
	userFundingThresholdsMutex.RLock(); defer userFundingThresholdsMutex.RUnlock()
	threshold, exists := userFundingThresholds[chatID]
	if !exists { return defaultFundingThreshold }; return threshold
}

func InitBot(token string) (*tgbotapi.BotAPI, error) {
	log.Println("Спроба ініціалізації бота через tgbotapi.NewBotAPI...")
	if token == "" { return nil, fmt.Errorf("токен бота порожній") }
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil { log.Printf("Помилка tgbotapi.NewBotAPI: %v", err); return nil, fmt.Errorf("NewBotAPI: %w", err) }
	if bot == nil { log.Printf("КРИТИЧНА ПОМИЛКА: bot is nil після NewBotAPI"); return nil, fmt.Errorf("bot is nil") }

	// Спроба обійти "mismatched types" шляхом прямого доступу до ID,
	// якщо bot.Self не nil (що має перевірятися неявно, якщо NewBotAPI успішний).
	// Якщо bot.Self *дійсно* не є *tgbotapi.User, це все одно не спрацює.
	var botID int64
	var userName string
	// Тимчасово прибираємо будь-яке пряме порівняння bot.Self з nil
	// Якщо bot.Self є nil, наступний рядок викличе паніку, але це допоможе ізолювати помилку компіляції.
	// ЦЕ ДУЖЕ РИЗИКОВАНО І ТІЛЬКИ ДЛЯ ДІАГНОСТИКИ "mismatched types"
	// Якщо компіляція пройде, АЛЕ буде паніка тут, це означає, що bot.Self - nil.
	// Якщо компіляція НЕ пройде з тією ж помилкою, проблема не тут.
	/*
	   if bot.Self == nil { // Якби компілятор це дозволяв...
	       log.Printf("КРИТИЧНА ПОМИЛКА: bot.Self is nil")
	       return nil, fmt.Errorf("bot.Self is nil")
	   }
	*/
	// Намагаємося просто використати, припускаючи, що NewBotAPI заповнив Self, якщо не було помилки
	botID = bot.Self.ID // Якщо тут паніка, значить bot.Self був nil
	userName = bot.Self.UserName

	if botID == 0 {
		log.Printf("ПОПЕРЕДЖЕННЯ: bot.Self.ID = 0. UserName: '%s'. Перевірте токен.", userName)
	} else {
		log.Printf("Бот успішно ініціалізований: ID=%d, UserName='%s'", botID, userName)
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

// SetWebhook: Ця версія очікує, що NewWebhook... ПОВЕРТАЄ ДВА ЗНАЧЕННЯ
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
	var errWebhookSetup error // Оголошуємо змінну для помилки

	if certFilePath != "" {
		fileBytes := tgbotapi.FilePath(certFilePath)
		whCfg, errWebhookSetup = tgbotapi.NewWebhookWithCert(fullWebhookURL, fileBytes) // Очікуємо два значення
	} else {
		whCfg, errWebhookSetup = tgbotapi.NewWebhook(fullWebhookURL) // Очікуємо два значення
	}

	if errWebhookSetup != nil { 
		log.Printf("ПОМИЛКА конфігурації вебхука NewWebhook...: %v", errWebhookSetup)
		return fmt.Errorf("конфігурація NewWebhook...: %w", errWebhookSetup)
	}

	whCfg.MaxConnections = 40
	_, err := bot.Request(whCfg) 
	if err != nil {
		log.Printf("ПОМИЛКА встановлення вебхука '%s': %v", fullWebhookURL, err)
		return fmt.Errorf("bot.Request(webhook setup): %w", err)
	}
	webhookInfo, errInfo := bot.GetWebhookInfo() 
	if errInfo != nil {
		log.Printf("ПОМИЛКА GetWebhookInfo: %v", errInfo)
		return fmt.Errorf("GetWebhookInfo: %w", errInfo)
	}
	if webhookInfo.IsSet() && webhookInfo.URL == fullWebhookURL {
		log.Printf("Вебхук успішно встановлено: URL='%s'", webhookInfo.URL)
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
	log.Println("Запит на видалення вебхука надіслано."); return nil
}
