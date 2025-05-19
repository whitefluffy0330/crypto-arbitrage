package main

import (
	"log"
	"net/http"
	// "strings" // Не використовується напряму тут
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	gsheetsService "github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // Використовуємо інший аліас, щоб уникнути конфлікту з gsheets
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation" // Закоментовано, якщо InitMotivationSeed не потрібен або викликається деінде
	// "golang.org/x/oauth2/google" // Не використовується напряму тут
	// "google.golang.org/api/option" // Не використовується напряму тут
	// gsheets "google.golang.org/api/sheets/v4" // Аліас gsheets вже не потрібен тут
)

// func appContext() context.Context { return context.Background() } // Не використовується напряму тут

func main() {
	// motivation.InitMotivationSeed() // Якщо InitMotivationSeed потрібен, розкоментуйте та переконайтеся, що пакет motivation імпортовано
	
	cfg, err := config.LoadEnv()
	if err != nil {
		log.Fatalf("Config load error: %v", err)
	}

	// Перевірка обов'язкових змінних з вашого оригінального main.go
	if cfg.TelegramToken == "" {
		log.Fatal("Критична помилка: TELEGRAM_TOKEN не встановлено!")
	}
	if cfg.SpreadsheetID == "" { // Це з вашого config.go
		log.Fatal("Критична помилка: SPREADSHEET_ID не встановлено!")
	}
	// if cfg.AdminChatID == 0 { // З вашого config.go, якщо це обов'язково
	// 	log.Fatal("Критична помилка: TELEGRAM_CHAT_ID не встановлено або 0!")
	// }


	svc, err := gsheetsService.NewService(cfg) // Використовуємо gsheetsService.NewService
	if err != nil {
		log.Fatalf("Sheets init error: %v", err)
	}

	bot, err := telegram.InitBot(cfg) // Передаємо *config.Config
	if err != nil {
		log.Fatalf("Telegram init error: %v", err)
	}

	// Згідно з вашим main.go, SetWebhook приймає (bot, certPath, url)
	// hook := cfg.WebhookBaseURL + cfg.WebhookPath // Це було у вас
	// if err := telegram.SetWebhook(bot, cfg.WebhookCertPath, hook); err != nil {
	// АЛЕ telegram.SetWebhook тепер приймає (bot, webhookBaseURL, webhookPath, certFilePath)
	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, cfg.WebhookPath, cfg.WebhookCertPath)
	if err != nil {
		log.Fatalf("Webhook set error: %v", err) // Змінено на Fatalf, як у вашому main.go
	}

	go func() {
		// bot.ListenForWebhook(cfg.WebhookPath) повертає UpdatesChannel, його результат має бути переданий в HandleUpdates
		// http.HandleFunc(cfg.WebhookPath, bot.ListenForWebhook) - так не працює, бо ListenForWebhook не є http.HandlerFunc
		// Правильний спосіб - передати nil як другий аргумент в ListenAndServe,
		// а ListenForWebhook має зареєструвати свій обробник на DefaultServeMux.
		// Це вже реалізовано в telegram.HandleUpdates через bot.ListenForWebhook()
		log.Printf("Listening for webhook on %s (шлях %s)", cfg.WebhookListenAddr, cfg.WebhookPath)
		if err := http.ListenAndServe(cfg.WebhookListenAddr, nil); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	telegram.HandleUpdates(bot, svc, cfg) // svc має бути *sheets.Service
}
