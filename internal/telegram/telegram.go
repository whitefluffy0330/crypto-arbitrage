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

// UserState представляє поточний стан діалогу користувача
type UserState string

const (
	StateDefault                 UserState = ""
	StateAwaitingGoalInput       UserState = "awaiting_goal_input"
	StateAwaitingInvestmentInput UserState = "awaiting_investment_input"
	StateAwaitingFundingThreshold UserState = "awaiting_funding_threshold"
)

var userStates = make(map[int64]UserState)
var userStatesMutex = &sync.Mutex{} // З вашого оригінального telegram.go (відповідь #62)

// FinancialGoal - структура для зберігання інформації про фінансову ціль користувача.
type FinancialGoal struct {
	Amount       float64
	Currency     string
	Days         int
	OriginalText string
	SetDate      time.Time
}

var (
	userGoals      = make(map[int64]FinancialGoal)
	userGoalsMutex sync.RWMutex // З вашого оригінального telegram.go (відповідь #62)
)

var (
	userFundingThresholds      = make(map[int64]float64)
	userFundingThresholdsMutex = &sync.Mutex{} // З вашого оригінального telegram.go (відповідь #62)
)

const defaultFundingThreshold = 0.0005 // З вашого оригінального telegram.go (відповідь #62)

func init() {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Printf("Крит. помилка telegram: не вдалося завантажити часову зону 'Europe/Kyiv': %v.", err)
		KyivLocation = time.UTC
	} else {
		KyivLocation = loc
		log.Println("Часову зону Europe/Kyiv завантажено (telegram init).")
	}
	// Ініціалізація глобальних змінних
	userStates = make(map[int64]UserState)
	userGoals = make(map[int64]FinancialGoal)
	userFundingThresholds = make(map[int64]float64)
}

func SetUserState(chatID int64, state UserState) { // З вашого telegram.go (відповідь #62), тип state UserState
	userStatesMutex.Lock()
	defer userStatesMutex.Unlock()
	userStates[chatID] = state
	log.Printf("Встановлено стан '%s' для ChatID %d", state, chatID)
}

func GetUserState(chatID int64) UserState { // З вашого telegram.go (відповідь #62), тип UserState
	userStatesMutex.Lock()
	defer userStatesMutex.Unlock()
	s, ok := userStates[chatID]
	if !ok {
		return StateDefault
	}
	return s
}

func SetUserGoal(chatID int64, goal FinancialGoal, srv *gsheets.Service, cfg config.Config) error { // З вашого telegram.go (відповідь #62)
	activeGoal, isActive := GetUserGoal(chatID, srv, cfg)
	if isActive {
		log.Printf("Для ChatID %d знайдено активну ціль (%+v) перед встановленням нової. Оновлюємо її статус на 'Перевизначено'.", chatID, activeGoal)
		errUpdateOld := sheets.UpdateGoalStatusInSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, "Перевизначено", time.Now().UTC())
		if errUpdateOld != nil {
			log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося оновити статус старої активної цілі на 'Перевизначено' для ChatID %d: %v", chatID, errUpdateOld)
		}
	}
	goalDataForSheet := sheets.FinancialGoalData{
		Amount:       goal.Amount,
		Currency:     goal.Currency,
		Days:         0,
		OriginalText: goal.OriginalText,
		SetDate:      goal.SetDate,
	}
	err := sheets.AddGoalToSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, goalDataForSheet)
	if err != nil {
		log.Printf("ПОМИЛКА запису нової цілі в Sheets для ChatID %d: %v", chatID, err)
		return fmt.Errorf("збереження нової цілі в Sheets: %w", err)
	}
	userGoalsMutex.Lock()
	userGoals[chatID] = goal
	userGoalsMutex.Unlock()
	log.Printf("Нову ціль успішно збережено та закешовано для ChatID %d: %+v", chatID, goal)
	return nil
}

func GetUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) (FinancialGoal, bool) { // З вашого telegram.go (відповідь #62)
	userGoalsMutex.RLock()
	goal, exists := userGoals[chatID]
	userGoalsMutex.RUnlock()
	if exists {
		return goal, true
	}
	log.Printf("Активна ціль для ChatID %d не знайдена в кеші. Спроба завантажити з Google Sheets (аркуш: %s).", chatID, cfg.SheetNameUserGoals)
	sheetGoalData, foundInSheet, err := sheets.GetActiveGoalFromSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID)
	if err != nil {
		log.Printf("ПОМИЛКА завантаження активної цілі з Sheets для ChatID %d: %v", chatID, err)
		return FinancialGoal{}, false
	}
	if foundInSheet {
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
		log.Printf("Активну ціль для ChatID %d успішно завантажено з Sheets та закешовано: %+v", chatID, loadedGoal)
		return loadedGoal, true
	}
	log.Printf("Активну ціль для ChatID %d не знайдено ні в кеші, ні в Sheets.", chatID)
	return FinancialGoal{}, false
}

func DeleteUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) error { // З вашого telegram.go (відповідь #62)
	log.Printf("Спроба закрити активну ціль для ChatID %d.", chatID)
	err := sheets.UpdateGoalStatusInSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, "Закрита", time.Now().UTC())
	if err != nil {
		if strings.Contains(err.Error(), "не знайдено активної цілі") {
			log.Printf("Немає активної цілі в Google Sheets для ChatID %d, щоб позначити як 'Закрита'.", chatID)
			ClearInMemoryUserGoal(chatID)
			return nil
		}
		log.Printf("ПОМИЛКА оновлення статусу цілі в Sheets для ChatID %d: %v", chatID, err)
		ClearInMemoryUserGoal(chatID)
		return fmt.Errorf("помилка оновлення статусу цілі в Google Sheets: %w", err)
	}
	ClearInMemoryUserGoal(chatID)
	log.Printf("Активну ціль для ChatID %d успішно позначено як 'Закрита' в Sheets та видалено з кешу.", chatID)
	return nil
}

func ClearInMemoryUserGoal(chatID int64) { // З вашого telegram.go (відповідь #62)
	userGoalsMutex.Lock()
	defer userGoalsMutex.Unlock()
	if _, exists := userGoals[chatID]; exists {
		delete(userGoals, chatID)
		log.Printf("Ціль для ChatID %d видалено з кешу.", chatID)
	}
}

func SetUserFundingThreshold(chatID int64, threshold float64) { // З вашого telegram.go (відповідь #62)
	fundingThresholdMutex.Lock()
	defer fundingThresholdMutex.Unlock()
	userFundingThresholds[chatID] = threshold
	log.Printf("Встановлено поріг фандингу %.4f%% для ChatID %d", threshold*100, chatID)
}

func GetUserFundingThreshold(chatID int64) float64 { // З вашого telegram.go (відповідь #62)
	fundingThresholdMutex.Lock() 
	defer fundingThresholdMutex.Unlock()
	if threshold, ok := userFundingThresholds[chatID]; ok {
		log.Printf("Для ChatID %d використовується поріг фандингу %.4f%%", chatID, threshold*100)
		return threshold
	}
	log.Printf("Для ChatID %d поріг фандингу не встановлено, використовується стандартний %.4f%%", chatID, defaultFundingThreshold*100)
	return defaultFundingThreshold
}

// InitBot: адаптовано до того, що компілятор бачить bot.Self як структуру User
func InitBot(cfg *config.Config) (*tgbotapi.BotAPI, error) { // Приймає *config.Config
	log.Println("Спроба ініціалізації бота...")
	if cfg.TelegramToken == "" { return nil, fmt.Errorf("токен бота порожній") }
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil { return nil, fmt.Errorf("NewBotAPI: %w", err) }
	if bot == nil { return nil, fmt.Errorf("bot nil після NewBotAPI") }

	// Якщо компілятор бачить bot.Self як структуру User, перевірка ID єдиний варіант.
	if bot.Self.ID == 0 {
		log.Printf("ПОПЕРЕДЖЕННЯ: bot.Self.ID = 0. UserName: '%s'. Спроба GetMe().", bot.Self.UserName)
		userInfo, errGetMe := bot.GetMe() // userInfo тут *tgbotapi.User
		if errGetMe != nil { return nil, fmt.Errorf("GetMe: %w", errGetMe) }
		
		// Обережна перевірка userInfo, який є *tgbotapi.User
		if userInfo == nil || userInfo.ID == 0 { 
			return nil, fmt.Errorf("GetMe: nil user або ID 0")
		}
		// Оновлюємо поля bot.Self (яке компілятор бачить як структуру)
		bot.Self.ID = userInfo.ID
		bot.Self.UserName = userInfo.UserName
		bot.Self.FirstName = userInfo.FirstName
		log.Printf("Дані оновлено через GetMe: ID=%d, UserName='%s'", bot.Self.ID, bot.Self.UserName)
	} else {
		log.Printf("Бот ініціалізований: ID=%d, UserName='%s'", bot.Self.ID, bot.Self.UserName)
	}
	return bot, nil
}

// SetWebhook: ОЧІКУЄМО ДВА ЗНАЧЕННЯ ВІД NewWebhook...
func SetWebhook(bot *tgbotapi.BotAPI, certPath, url string) error { // Змінено порядок аргументів як у вашому main.go
	if url == "" { // Перевірка на порожній URL
		log.Println("ПОПЕРЕДЖЕННЯ: URL для вебхука не вказано.")
		return nil // Або повернути помилку, якщо URL обов'язковий
	}
	log.Printf("Встановлення вебхука: URL=%s, CertFile (якщо є)=%s", url, certPath)
	if !strings.HasPrefix(url, "https://") {
		log.Printf("ПОПЕРЕДЖЕННЯ: URL вебхука '%s' не починається з https://.", url)
	}

	var whCfg tgbotapi.WebhookConfig
	var errWebhookSetup error 

	if certPath != "" {
		fileBytes := tgbotapi.FilePath(certPath)
		whCfg, errWebhookSetup = tgbotapi.NewWebhookWithCert(url, fileBytes) 
	} else {
		whCfg, errWebhookSetup = tgbotapi.NewWebhook(url) 
	}

	if errWebhookSetup != nil { 
		log.Printf("ПОМИЛКА конфігурації вебхука NewWebhook...: %v", errWebhookSetup)
		return fmt.Errorf("конфігурація NewWebhook...: %w", errWebhookSetup)
	}

	whCfg.MaxConnections = 40 // Як було у вашому оригінальному telegram.go
	resp, err := bot.Request(whCfg) 
	if err != nil {
		log.Printf("ПОМИЛКА встановлення вебхука '%s': %v", url, err)
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
	if webhookInfo.IsSet() && webhookInfo.URL == url { // Порівнюємо з url
		log.Printf("Вебхук успішно встановлено: URL='%s'", webhookInfo.URL)
		if webhookInfo.LastErrorDate != 0 {
			log.Printf("  Остання помилка вебхука: %s (дата: %s)", webhookInfo.LastErrorMessage, time.Unix(int64(webhookInfo.LastErrorDate), 0).Format(time.RFC3339))
		}
		if webhookInfo.IPAddress != "" {
			log.Printf("  IP-адреса вебхука (як бачить Telegram): %s", webhookInfo.IPAddress)
		}
	} else {
		log.Printf("ПОПЕРЕДЖЕННЯ: Вебхук НЕ встановлено. URL: '%s', Очікувано: '%s'", webhookInfo.URL, url)
		return fmt.Errorf("вебхук не встановлено: URL '%s' != '%s'", webhookInfo.URL, url)
	}
	return nil
}

// HandleUpdates викликає HandleUpdate (яка має бути визначена в handler.go)
func HandleUpdates(bot *tgbotapi.BotAPI, svc *sheets.Service, cfg *config.Config) { // Змінено тип svc на *sheets.Service
	updates := bot.ListenForWebhook(cfg.WebhookPath)
	log.Println("Розпочато обробку оновлень Telegram (з telegram.go)...")
	for update := range updates {
		go HandleUpdate(bot, update, svc, cfg) 
	}
	log.Println("Зупинено обробку оновлень Telegram (канал закрито).")
}

func RemoveWebhook(bot *tgbotapi.BotAPI) error { // З вашого оригінального telegram.go
	log.Println("Спроба видалення вебхука...")
	// tgbotapi.DeleteWebhookConfig{} був у вашому оригіналі, але нова версія може вимагати DropPendingUpdates
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
