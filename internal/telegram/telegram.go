package telegram

import (
	"log"
	"sync" // Для sync.RWMutex

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Використовуємо аліас gsheets, щоб було зрозуміло, що srv - це сервіс Google API
	gsheets "google.golang.org/api/sheets/v4"
)

// userGoals зберігатиме цілі користувачів.
// Ключ: chatID, Значення: опис цілі (string) або ваша структура для цілі.
// Доступ до цієї мапи має бути синхронізований.
var (
	userGoals      = make(map[int64]string) // Приклад: map[chatID]goalDescription.
	userGoalsMutex sync.RWMutex
)

// InitBot тепер просто створює та повертає новий екземпляр бота.
// Налаштування Webhook та отримання оновлень буде оброблятися в main.go.
func InitBot(token string) (*tgbotapi.BotAPI, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Помилка створення екземпляра бота: %v", err)
		return nil, err
	}
	// Можна залишити логування тут або перенести в main.go після успішної ініціалізації
	// log.Printf("✅ Telegram бот екземпляр створено для @%s", bot.Self.UserName)
	return bot, nil
}

// HandleUpdates отримує оновлення з каналу (який надається слухачем вебхуків у main.go)
// та передає кожне оновлення до HandleUpdate (в однині, визначеної в handler.go).
func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *gsheets.Service, spreadsheetID string) {
	log.Println("Розпочато обробку оновлень...")
	for update := range updates {
		// Викликаємо HandleUpdate (визначену в handler.go, той самий пакет)
		// з правильним порядком параметрів.
		HandleUpdate(bot, update, srv, spreadsheetID)
	}
	log.Println("Зупинено обробку оновлень (канал закрито).")
}

// SetWebhook - допоміжна функція для встановлення вебхука для бота.
// Може викликатися з main.go.
// webhookBaseURL має бути вигляду "https://yourdomain.com"
// webhookPath має бути вигляду "/your_secret_webhook_path"
// certFilePath - шлях до вашого публічного SSL-сертифіката (для самопідписаних сертифікатів,
// або якщо Telegram цього вимагає). Для Let's Encrypt зазвичай не потрібно передавати сертифікат,
// якщо ваш HTTPS-сервер правильно налаштований.
func SetWebhook(bot *tgbotapi.BotAPI, webhookBaseURL string, webhookPath string, certFilePath string) error {
	fullWebhookURL := webhookBaseURL + webhookPath
	log.Printf("Встановлення вебхука на: %s", fullWebhookURL)

	var whCfg tgbotapi.WebhookConfig
	var errWh error // Оголошуємо змінну для помилки від NewWebhook*

	if certFilePath != "" {
		// Використовуйте це, якщо потрібно завантажити файл публічного сертифіката в Telegram
		whCfg, errWh = tgbotapi.NewWebhookWithCert(fullWebhookURL, tgbotapi.FilePath(certFilePath))
	} else {
		// Використовуйте це, якщо ваш HTTPS-ендпоінт налаштований з сертифікатом від надійного CA
		whCfg, errWh = tgbotapi.NewWebhook(fullWebhookURL)
	}

	// Перевіряємо помилку після виклику NewWebhook*
	if errWh != nil {
		log.Printf("Помилка створення конфігурації вебхука: %v", errWh)
		return errWh // Повертаємо помилку
	}

	// Тут можна встановити AllowedUpdates, якщо ви хочете фільтрувати типи оновлень
	// whCfg.AllowedUpdates = []string{"message", "callback_query"}

	// Використовуємо нову змінну для помилки від bot.Request, щоб не затерти errWh
	_, errReq := bot.Request(whCfg)
	if errReq != nil {
		log.Printf("Помилка встановлення вебхука (bot.Request): %v", errReq)
		return errReq
	}

	info, errInfo := bot.GetWebhookInfo()
	if errInfo != nil {
		log.Printf("Помилка отримання інформації про вебхук: %v", errInfo)
		// Можна не повертати помилку тут, оскільки вебхук міг встановитися,
		// але інформацію отримати не вдалося. Це не критично для роботи.
	} else {
		if info.LastErrorDate != 0 {
			log.Printf("Помилка останнього зворотного виклику Telegram (вебхук): %s. URL: %s", info.LastErrorMessage, info.URL)
		} else if info.URL == "" {
			// Це може статися, якщо вебхук був видалений або ще не повністю встановлений
			log.Printf("Вебхук оброблено, але URL порожній. Перевірте налаштування або GetWebhookInfo() пізніше.")
		} else {
			log.Printf("Вебхук успішно встановлено. URL: %s", info.URL)
		}
	}
	return nil
}

// RemoveWebhook може використовуватися для видалення вебхука (наприклад, для тестування long polling локально)
func RemoveWebhook(bot *tgbotapi.BotAPI) error {
	_, err := bot.Request(tgbotapi.DeleteWebhookConfig{})
	if err != nil {
		log.Printf("Помилка видалення вебхука: %v", err)
		return err
	}
	log.Println("Вебхук успішно видалено.")
	return nil
}

// TODO: Далі можуть бути функції для роботи з userGoals, наприклад:
// func GetUserGoal(chatID int64) (string, bool) {
//	 userGoalsMutex.RLock()
//	 defer userGoalsMutex.RUnlock()
//	 goal, exists := userGoals[chatID]
//	 return goal, exists
// }
//
// func SetUserGoal(chatID int64, goalText string) {
//	 userGoalsMutex.Lock()
//	 defer userGoalsMutex.Unlock()
//	 userGoals[chatID] = goalText
// }
//
// func DeleteUserGoal(chatID int64) {
//	 userGoalsMutex.Lock()
//	 defer userGoalsMutex.Unlock()
//	 delete(userGoals, chatID)
// }
// Ми реалізуємо їх пізніше, коли будемо працювати над логікою цілей.
