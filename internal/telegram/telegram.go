package telegram

import (
	"log"
	"sync" // Для sync.RWMutex

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Імпорт google.golang.org/api/sheets/v4 потрібен тут,
	// якщо HandleUpdates або інші функції цього пакету безпосередньо використовують тип sheets.Service
	// або його методи. Наразі він використовується для типу параметра srv.
	"google.golang.org/api/sheets/v4"
)

// userGoals зберігатиме цілі користувачів.
// Ключ: chatID, Значення: опис цілі (string) або структура цілі.
// Доступ до цієї мапи має бути синхронізований.
var (
	userGoals      = make(map[int64]string) // Приклад: map[chatID]goalDescription
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
	log.Printf("✅ Telegram бот екземпляр створено для @%s", bot.Self.UserName)
	return bot, nil
}

// HandleUpdates отримує оновлення з каналу (який надається слухачем вебхуків у main.go)
// та передає кожне оновлення до HandleUpdate (в однині).
func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *sheets.Service, spreadsheetID string) {
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
	if certFilePath != "" {
		// Використовуйте це, якщо потрібно завантажити файл публічного сертифіката в Telegram
		// (наприклад, для самопідписаних сертифікатів).
		whCfg = tgbotapi.NewWebhookWithCert(fullWebhookURL, tgbotapi.FilePath(certFilePath))
	} else {
		// Використовуйте це, якщо ваш HTTPS-ендпоінт налаштований з сертифікатом від надійного CA (наприклад, Let's Encrypt).
		whCfg = tgbotapi.NewWebhook(fullWebhookURL)
	}

	// Тут можна встановити AllowedUpdates, якщо ви хочете фільтрувати типи оновлень
	// whCfg.AllowedUpdates = []string{"message", "callback_query"}

	_, err := bot.Request(whCfg)
	if err != nil {
		log.Printf("Помилка встановлення вебхука: %v", err)
		return err
	}

	// Опціонально, перевірити інформацію про вебхук
	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Printf("Помилка отримання інформації про вебхук: %v", err)
		// Не повертаємо помилку тут, оскільки вебхук міг встановитися, але інформацію отримати не вдалося
	} else {
		if info.LastErrorDate != 0 {
			log.Printf("Помилка зворотного виклику Telegram (вебхук): %s. URL: %s", info.LastErrorMessage, info.URL)
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
