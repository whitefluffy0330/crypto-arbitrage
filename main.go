package main

import (
	"context"
	"log"
	"net/http"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation" 

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5" // Імпорт

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4"
)

func appContext() context.Context {
	return context.Background()
}

func main() {
	motivation.InitMotivationSeed()

	// ДІАГНОСТИЧНИЙ РЯДОК: Додано для вирішення помилки "imported and not used"
	var _ tgbotapi.Update // Переконуємося, що тип з пакета tgbotapi використовується

	cfg := config.LoadEnv() 

	if cfg.BotToken == "" || cfg.SpreadsheetID == "" {
		log.Fatal("Критична помилка: Не задані обов'язкові змінні середовища TELEGRAM_TOKEN та SPREADSHEET_ID")
	}

	bot, err := telegram.InitBot(cfg.BotToken) // Використовує *tgbotapi.BotAPI
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	log.Printf("Бот @%s ініціалізовано.", bot.Self.UserName) // Використовує bot.Self (*tgbotapi.User)

	webhookBaseURL := "https://vadymnewchapter.pp.ua"
	webhookPath := "/webhook_" + bot.Token // Використовує bot.Token
	certFilePath := "" 

	err = telegram.SetWebhook(bot, webhookBaseURL, webhookPath, certFilePath) // Передає bot
	if err != nil {
		log.Fatalf("Помилка встановлення вебхука: %v", err)
	}

	ctx := appContext()
	credentials, err := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope) 
	if err != nil {
		log.Fatalf("Помилка авторизації Google Sheets (перевірте GOOGLE_APPLICATION_CREDENTIALS): %v", err)
	}

	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err)
	}

	// Використовує bot.ListenForWebhook та тип tgbotapi.UpdatesChannel
	updates := bot.ListenForWebhook(webhookPath) 

	go func() {
		log.Printf("Запуск HTTPS сервера для вебхука на порту 443, шлях: %s", webhookPath)
		err_https := http.ListenAndServeTLS(":443",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem",
			nil)
		if err_https != nil {
			log.Fatalf("Помилка запуску HTTPS сервера: %v", err_https)
		}
	}()

	log.Printf("Бот @%s готовий до роботи та очікує на оновлення через вебхук...", bot.Self.UserName) // Використовує bot.Self

	// telegram.StartEveningReport(bot, sheetsService, cfg) 

	// Передає updates (tgbotapi.UpdatesChannel) та bot (*tgbotapi.BotAPI)
	telegram.HandleUpdates(updates, bot, sheetsService, cfg)
}
