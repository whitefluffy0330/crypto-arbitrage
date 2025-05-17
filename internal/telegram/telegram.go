package telegram

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5" // Переконайтеся, що цей імпорт правильний
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4"
)

var KyivLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Printf("Крит. помилка telegram: не вдалося завантажити часову зону 'Europe/Kyiv': %v.", err)
		KyivLocation = time.UTC // Відкат до UTC у разі помилки
	} else {
		KyivLocation = loc
		log.Println("Часову зону Europe/Kyiv завантажено (telegram).")
	}
}

// FinancialGoal - структура для зберігання інформації про фінансову ціль користувача.
type FinancialGoal struct {
	Amount       float64
	Currency     string
	Days         int // Це поле використовувалося для старої логіки цілей, може бути неактуальним для місячних
	OriginalText string
	SetDate      time.Time // Дата встановлення цілі, використовується для визначення місяця цілі
}

var (
	userGoals      = make(map[int64]FinancialGoal) // Кеш активних цілей користувачів
	userGoalsMutex sync.RWMutex
)

var (
	userStates      = make(map[int64]string) // Поточний стан користувача для покрокових команд
	userStatesMutex sync.RWMutex
)

// Константи для станів користувача
const (
	StateDefault                 = "" // Стан за замовчуванням, немає активної операції
	StateAwaitingGoalInput       = "awaiting_goal_input"
	StateAwaitingInvestmentInput = "awaiting_investment_input"
	StateAwaitingFundingThreshold = "awaiting_funding_threshold"
)

var (
	userFundingThresholds      = make(map[int64]float64) // Кеш порогів фандингу для користувачів
	userFundingThresholdsMutex sync.RWMutex
)

const defaultFundingThreshold = 0.0005 // Поріг фандингу за замовчуванням (0.05%)

// SetUserState встановлює або скидає стан для вказаного ChatID.
func SetUserState(chatID int64, state string) {
	userStatesMutex.Lock()
	defer userStatesMutex.Unlock()
	if state == StateDefault {
		delete(userStates, chatID)
	} else {
		userStates[chatID] = state
	}
	log.Printf("Встановлено стан '%s' для ChatID %d", state, chatID)
}

// GetUserState повертає поточний стан для вказаного ChatID.
func GetUserState(chatID int64) string {
	userStatesMutex.RLock()
	defer userStatesMutex.RUnlock()
	state, exists := userStates[chatID]
	if !exists {
		return StateDefault
	}
	return state
}

// SetUserGoal зберігає нову фінансову ціль користувача, оновлює статус старої активної цілі в Google Sheets,
// записує нову ціль у Google Sheets та кешує її.
func SetUserGoal(chatID int64, goal FinancialGoal, srv *gsheets.Service, cfg config.Config) error {
	// Перевіряємо, чи є вже активна ціль, і якщо так, оновлюємо її статус
	activeGoal, isActive := GetUserGoal(chatID, srv, cfg) // Використовуємо GetUserGoal, щоб отримати з кешу або Sheets
	if isActive {
		log.Printf("Для ChatID %d знайдено активну ціль (%+v) перед встановленням нової. Оновлюємо її статус на 'Перевизначено'.", chatID, activeGoal)
		// Оновлюємо статус старої цілі в Google Sheets
		errUpdateOld := sheets.UpdateGoalStatusInSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, "Перевизначено", time.Now().UTC())
		if errUpdateOld != nil {
			// Логуємо попередження, але не перериваємо встановлення нової цілі
			log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося оновити статус старої активної цілі на 'Перевизначено' для ChatID %d: %v", chatID, errUpdateOld)
		}
	}

	// Підготовка даних для запису в Google Sheets
	goalDataForSheet := sheets.FinancialGoalData{
		Amount:       goal.Amount,
		Currency:     goal.Currency,
		Days:         0, // Для місячних цілей це поле може не використовуватися або позначати тривалість
		OriginalText: goal.OriginalText,
		SetDate:      goal.SetDate, // Зберігаємо дату встановлення
	}

	// Додаємо нову ціль у Google Sheets
	err := sheets.AddGoalToSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, goalDataForSheet)
	if err != nil {
		log.Printf("ПОМИЛКА запису нової цілі в Sheets для ChatID %d: %v", chatID, err)
		return fmt.Errorf("збереження нової цілі в Sheets: %w", err)
	}

	// Оновлюємо кеш цілей
	userGoalsMutex.Lock()
	userGoals[chatID] = goal
	userGoalsMutex.Unlock()
	log.Printf("Нову ціль успішно збережено та закешовано для ChatID %d: %+v", chatID, goal)
	return nil
}

// GetUserGoal повертає активну фінансову ціль для користувача з кешу або Google Sheets.
func GetUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) (FinancialGoal, bool) {
	userGoalsMutex.RLock()
	goal, exists := userGoals[chatID]
	userGoalsMutex.RUnlock()

	if exists {
		// Перевірка, чи ціль все ще актуальна для поточного місяця (якщо потрібно)
		// Наприклад, якщо SetDate не в поточному місяці, можна вважати її неактивною
		// Або покладатися на статус "Активна" з Google Sheets.
		// Поки що, якщо є в кеші, вважаємо активною.
		return goal, true
	}

	// Якщо в кеші немає, намагаємося завантажити з Google Sheets
	log.Printf("Активна ціль для ChatID %d не знайдена в кеші. Спроба завантажити з Google Sheets (аркуш: %s).", chatID, cfg.SheetNameUserGoals)
	sheetGoalData, foundInSheet, err := sheets.GetActiveGoalFromSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID)
	if err != nil {
		log.Printf("ПОМИЛКА завантаження активної цілі з Sheets для ChatID %d: %v", chatID, err)
		return FinancialGoal{}, false // Повертаємо, що ціль не знайдена
	}

	if foundInSheet {
		// Створюємо об'єкт FinancialGoal з даних, отриманих з Sheets
		loadedGoal := FinancialGoal{
			Amount:       sheetGoalData.Amount,
			Currency:     sheetGoalData.Currency,
			Days:         sheetGoalData.Days,         // Це поле з Sheets
			OriginalText: sheetGoalData.OriginalText, // Це поле з Sheets
			SetDate:      sheetGoalData.SetDate,      // Це поле з Sheets
		}
		// Зберігаємо завантажену ціль у кеш
		userGoalsMutex.Lock()
		userGoals[chatID] = loadedGoal
		userGoalsMutex.Unlock()
		log.Printf("Активну ціль для ChatID %d успішно завантажено з Sheets та закешовано: %+v", chatID, loadedGoal)
		return loadedGoal, true
	}

	log.Printf("Активну ціль для ChatID %d не знайдено ні в кеші, ні в Sheets.", chatID)
	return FinancialGoal{}, false
}

// DeleteUserGoal оновлює статус активної цілі на "Закрита" в Google Sheets та видаляє її з кешу.
func DeleteUserGoal(chatID int64, srv *gsheets.Service, cfg config.Config) error {
	log.Printf("Спроба закрити активну ціль для ChatID %d.", chatID)
	// Оновлюємо статус в Google Sheets
	err := sheets.UpdateGoalStatusInSheet(srv, cfg.SpreadsheetID, cfg.SheetNameUserGoals, chatID, "Закрита", time.Now().UTC())
	if err != nil {
		// Якщо помилка містить "не знайдено активної цілі", це означає, що в Sheets вже немає активної цілі.
		// В цьому випадку ми все одно повинні очистити кеш.
		if strings.Contains(err.Error(), "не знайдено активної цілі") {
			log.Printf("Немає активної цілі в Google Sheets для ChatID %d, щоб позначити як 'Закрита'. Очищаємо кеш.", chatID)
			ClearInMemoryUserGoal(chatID) // Очищаємо кеш
			return nil // Не повертаємо помилку, оскільки стан консистентний
		}
		// Інша помилка при оновленні Sheets
		log.Printf("ПОМИЛКА оновлення статусу цілі в Sheets для ChatID %d: %v", chatID, err)
		// Навіть якщо сталася помилка в Sheets, спробуємо очистити кеш, щоб уникнути неконсистентності
		ClearInMemoryUserGoal(chatID)
		return fmt.Errorf("помилка оновлення статусу цілі в Google Sheets: %w", err)
	}

	// Якщо в Sheets все оновилося успішно, очищаємо кеш
	ClearInMemoryUserGoal(chatID)
	log.Printf("Активну ціль для ChatID %d успішно позначено як 'Закрита' в Sheets та видалено з кешу.", chatID)
	return nil
}

// ClearInMemoryUserGoal видаляє ціль користувача з кешу.
func ClearInMemoryUserGoal(chatID int64) {
	userGoalsMutex.Lock()
	defer userGoalsMutex.Unlock()
	if _, exists := userGoals[chatID]; exists {
		delete(userGoals, chatID)
		log.Printf("Ціль для ChatID %d видалено з кешу.", chatID)
	} else {
		log.Printf("Ціль для ChatID %d вже відсутня в кеші, нічого видаляти.", chatID)
	}
}

// SetUserFundingThreshold встановлює поріг фандингу для користувача.
func SetUserFundingThreshold(chatID int64, threshold float64) {
	userFundingThresholdsMutex.Lock()
	defer userFundingThresholdsMutex.Unlock()
	userFundingThresholds[chatID] = threshold
	log.Printf("Встановлено поріг фандингу %.4f%% для ChatID %d", threshold*100, chatID) // Множимо на 100 для відображення у %
}

// GetUserFundingThreshold повертає поріг фандингу для користувача.
// Якщо не встановлено, повертає значення за замовчуванням.
func GetUserFundingThreshold(chatID int64) float64 {
	userFundingThresholdsMutex.RLock()
	defer userFundingThresholdsMutex.RUnlock()
	threshold, exists := userFundingThresholds[chatID]
	if !exists {
		log.Printf("Для ChatID %d поріг фандингу не встановлено, використовується стандартний %.4f%%", chatID, defaultFundingThreshold*100)
		return defaultFundingThreshold
	}
	log.Printf("Для ChatID %d використовується поріг фандингу %.4f%%", chatID, threshold*100)
	return threshold
}

// InitBot ініціалізує та повертає екземпляр бота.
// !!! ВИПРАВЛЕНО ПЕРЕВІРКУ bot.Self !!!
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

	// Важлива перевірка: спочатку перевіряємо, чи bot.Self не nil
	if bot.Self == nil {
		log.Printf("КРИТИЧНА ПОМИЛКА ІНІЦІАЛІЗАЦІЇ: bot.Self є nil після NewBotAPI. Можливо, невалідний токен або серйозна проблема з API Telegram.")
		return nil, fmt.Errorf("bot.Self is nil after NewBotAPI, token might be invalid or Telegram API issue")
	}

	// Тепер, коли ми знаємо, що bot.Self не nil, можна безпечно доступатися до його полів
	if bot.Self.ID == 0 {
		log.Printf("ПОПЕРЕДЖЕННЯ: bot.Self.ID = 0 після NewBotAPI, хоча bot.Self не nil. UserName: '%s'. Перевірте токен.", bot.Self.UserName)
	} else {
		log.Printf("Бот успішно ініціалізований: ID=%d, UserName='%s'", bot.Self.ID, bot.Self.UserName)
	}
	return bot, nil
}


// HandleUpdates отримує оновлення з каналу та передає їх в HandleUpdate.
func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, cfg config.Config) {
	log.Println("Розпочато обробку оновлень Telegram...")
	for update := range updates {
		// Кожне оновлення обробляється в окремій горутині для уникнення блокування
		// Тут можна додати sync.WaitGroup, якщо потрібно чекати завершення всіх обробок перед виходом
		go HandleUpdate(bot, update, srv, cfg)
	}
	log.Println("Зупинено обробку оновлень Telegram (канал закрито).")
}

// SetWebhook встановлює вебхук для бота.
// Якщо certFilePath порожній, вебхук встановлюється без сертифіката (для роботи за Nginx).
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error {
	if webhookBaseURL == "" || webhookPath == "" {
		log.Println("ПОПЕРЕДЖЕННЯ: WebhookBaseURL або WebhookPath не вказані. Вебхук не буде встановлено.")
		// Можна спробувати видалити існуючий вебхук, якщо він був
		// removeErr := RemoveWebhook(bot)
		// if removeErr != nil {
		// 	log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося видалити існуючий вебхук: %v", removeErr)
		// }
		return nil // Не встановлюємо вебхук, якщо немає URL/Path
	}

	log.Printf("Встановлення вебхука: URL=%s%s, CertFile (якщо є)=%s", webhookBaseURL, webhookPath, certFilePath)
	fullWebhookURL := webhookBaseURL + webhookPath
	if !strings.HasPrefix(fullWebhookURL, "https://") {
		log.Printf("ПОПЕРЕДЖЕННЯ: URL вебхука '%s' не починається з https://. Це може не спрацювати.", fullWebhookURL)
	}

	var whCfg tgbotapi.WebhookConfig
	var errWebhookSetup error

	if certFilePath != "" {
		log.Printf("Спроба встановити вебхук з файлом сертифіката: %s", certFilePath)
		// tgbotapi.FilePath(certFilePath) - це спосіб передати шлях до файлу сертифіката
		fileBytes := tgbotapi.FilePath(certFilePath)
		whCfg = tgbotapi.NewWebhookWithCert(fullWebhookURL, fileBytes)
		// NewWebhookWithCert не повертає помилку напряму, помилка буде при bot.Request
	} else {
		log.Printf("Спроба встановити вебхук без файлу сертифіката (Nginx має обробляти SSL).")
		whCfg = tgbotapi.NewWebhook(fullWebhookURL)
		// NewWebhook також не повертає помилку напряму
	}

	// Можна встановити додаткові параметри для вебхука
	whCfg.MaxConnections = 40 // Максимальна кількість одночасних підключень
	// whCfg.AllowedUpdates = []string{"message", "callback_query"} // Які типи оновлень отримувати

	// Надсилаємо запит на встановлення вебхука
	_, err := bot.Request(whCfg)
	if err != nil {
		log.Printf("ПОМИЛКА встановлення вебхука '%s': %v", fullWebhookURL, err)
		return fmt.Errorf("bot.Request(webhook setup) failed: %w", err)
	}

	// Перевіряємо інформацію про встановлений вебхук
	webhookInfo, err := bot.GetWebhookInfo()
	if err != nil {
		log.Printf("ПОМИЛКА отримання інформації про вебхук після встановлення: %v", err)
		return fmt.Errorf("bot.GetWebhookInfo failed after setup: %w", err)
	}

	if webhookInfo.IsSet() && webhookInfo.URL == fullWebhookURL {
		log.Printf("Вебхук успішно встановлено: URL='%s', MaxConnections=%d, PendingUpdates=%d",
			webhookInfo.URL, webhookInfo.MaxConnections, webhookInfo.PendingUpdateCount)
		if webhookInfo.LastErrorDate != 0 {
			log.Printf("  Остання помилка вебхука: %s (дата: %s)", webhookInfo.LastErrorMessage, time.Unix(int64(webhookInfo.LastErrorDate), 0).Format(time.RFC3339))
		}
		if webhookInfo.IPAddress != "" {
			log.Printf("  IP-адреса вебхука (як бачить Telegram): %s", webhookInfo.IPAddress)
		}
	} else {
		log.Printf("ПОПЕРЕДЖЕННЯ: Вебхук НЕ встановлено належним чином після запиту. Поточний URL: '%s', Очікуваний: '%s'", webhookInfo.URL, fullWebhookURL)
		return fmt.Errorf("webhook URL ('%s') was not set to the expected ('%s') by Telegram after request", webhookInfo.URL, fullWebhookURL)
	}
	return nil
}

// RemoveWebhook видаляє поточний вебхук.
func RemoveWebhook(bot *tgbotapi.BotAPI) error {
	log.Println("Спроба видалення вебхука...")
	// Передаємо true для drop_pending_updates, щоб очистити чергу оновлень
	_, err := bot.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: true})
	if err != nil {
		log.Printf("ПОМИЛКА видалення вебхука: %v", err)
		return fmt.Errorf("bot.Request(delete webhook) failed: %w", err)
	}
	log.Println("Запит на видалення вебхука надіслано.")

	// Перевіряємо інформацію після видалення
	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Printf("ПОМИЛКА отримання інформації про вебхук після запиту на видалення: %v", err)
		// Не повертаємо помилку, оскільки запит на видалення вже пройшов
	} else if info.IsSet() && info.URL != "" {
		log.Printf("ПОПЕРЕДЖЕННЯ: Вебхук все ще встановлений на URL: '%s' після запиту на видалення. Можливо, потрібно зачекати.", info.URL)
	} else {
		log.Println("Вебхук успішно видалено (або не був встановлений).")
	}
	return nil
}
