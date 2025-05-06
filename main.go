package main

import (
	"context"
	"log"
	"net/http"
	// "time" // Можливо, time вже не потрібен тут безпосередньо

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // Для sheets.SpreadsheetsScope
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	// Імпортуйте пакет motivation, якщо InitMotivationSeed буде тут
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"


	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4" // Перейменовано, щоб уникнути конфлікту з вашим пакетом sheets
)

// appContext залишається без змін, якщо потрібен
func appContext() context.Context {
	return context.Background()
}

func main() {
	cfg := config.LoadEnv()

	// Ініціалізація насіння для мотиваційних фраз
	motivation.InitMotivationSeed() // <--- ВАЖЛИВО: Додайте цей виклик

	if cfg.BotToken == "" || cfg.SpreadsheetID == "" || cfg.ChatID == 0 { // ChatID може бути не потрібен для загального запуску, а для конкретних повідомлень
		log.Fatal("Не задані обов'язкові змінні середовища (TELEGRAM_TOKEN, SPREADSHEET_ID)")
	}

	bot, err := telegram.InitBot(cfg.BotToken) // Використовуємо оновлену InitBot
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}

	// Налаштування та встановлення Webhook
	// Ці значення мають надходити з конфігурації або бути визначені
	webhookBaseURL := "https://vadymnewchapter.pp.ua" // ВАШ ДОМЕН
	webhookPath := "/webhook_" + bot.Token            // Унікальний шлях, щоб ніхто інший його не знав
	// certFilePath := "/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem" // Потрібно, лише якщо NewWebhookWithCert
	certFilePath := "" // Залиште порожнім, якщо у вас Let's Encrypt і сервер налаштований правильно

	// Встановлюємо вебхук (використовуючи функцію з пакета telegram)
	err = telegram.SetWebhook(bot, webhookBaseURL, webhookPath, certFilePath)
	if err != nil {
		log.Fatalf("Помилка встановлення вебхука: %v", err)
	}

	ctx := appContext()
	// Переконайтеся, що змінна середовища GOOGLE_APPLICATION_CREDENTIALS встановлена правильно
	credentials, err := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope) // sheets.SpreadsheetsScope з вашого пакету
	if err != nil {
		log.Fatalf("Помилка авторизації Google Sheets (перевірте GOOGLE_APPLICATION_CREDENTIALS): %v", err)
	}

	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials)) // Використовуємо gsheets.NewService
	if err != nil {
		log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err)
	}

	// Слухаємо оновлення, що надходять на вебхук
	// ListenForWebhook реєструє обробник на http.DefaultServeMux
	updates := bot.ListenForWebhook(webhookPath) // Шлях має точно співпадати з тим, що встановлено у SetWebhook

	// Запуск HTTPS сервера для вебхука
	go func() {
		log.Printf("Запуск HTTPS сервера для вебхука на порту 443, шлях: %s", webhookPath)
		// Шляхи до сертифікатів Let's Encrypt - переконайтеся, що вони правильні
		// і доступні для читання вашим застосунком.
		// Розгляньте можливість винесення шляхів до сертифікатів у конфігурацію.
		err := http.ListenAndServeTLS(":443", // Або інший порт, якщо у вас є реверс-проксі типу Nginx
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem",
			nil) // nil означає використання http.DefaultServeMux, де ListenForWebhook реєструє свій обробник
		if err != nil {
			log.Fatalf("Помилка запуску HTTPS сервера: %v", err)
		}
	}()

	// telegram.StartEveningReport(bot, sheetsService, cfg.SpreadsheetID, cfg.ChatID) // Цю функцію ми ще не бачили

	// Передаємо cfg.SpreadsheetID замість всього cfg
	telegram.HandleUpdates(updates, bot, sheetsService, cfg.SpreadsheetID)
}
