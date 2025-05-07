package main

import (
	"context"
	"log"
	"net/http"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config" // Імпортуємо конфігурацію
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // Для sheets.SpreadsheetsScope
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation" 

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4"
)

func appContext() context.Context {
	return context.Background()
}

func main() {
	motivation.InitMotivationSeed()

	cfg := config.LoadEnv() // Завантажуємо всю конфігурацію

	// Перевіряємо лише найкритичніші параметри для запуску
	if cfg.BotToken == "" || cfg.SpreadsheetID == "" {
		log.Fatal("Критична помилка: Не задані обов'язкові змінні середовища TELEGRAM_TOKEN та SPREADSHEET_ID")
	}

	bot, err := telegram.InitBot(cfg.BotToken)
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	log.Printf("Бот @%s ініціалізовано.", bot.Self.UserName)

	// TODO: Винести webhookBaseURL, webhookPath, certFilePath у cfg
	webhookBaseURL := "https://vadymnewchapter.pp.ua"
	webhookPath := "/webhook_" + bot.Token
	certFilePath := "" 

	err = telegram.SetWebhook(bot, webhookBaseURL, webhookPath, certFilePath)
	if err != nil {
		log.Fatalf("Помилка встановлення вебхука: %v", err)
	}

	ctx := appContext()
	credentials, err := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope) // sheets.SpreadsheetsScope з вашого пакета
	if err != nil {
		log.Fatalf("Помилка авторизації Google Sheets (перевірте GOOGLE_APPLICATION_CREDENTIALS): %v", err)
	}

	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err)
	}

	updates := bot.ListenForWebhook(webhookPath)

	go func() {
		log.Printf("Запуск HTTPS сервера для вебхука на порту 443, шлях: %s", webhookPath)
		// TODO: Винести шляхи до сертифікатів у cfg
		err_https := http.ListenAndServeTLS(":443",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem",
			nil)
		if err_https != nil {
			log.Fatalf("Помилка запуску HTTPS сервера: %v", err_https)
		}
	}()

	log.Printf("Бот @%s готовий до роботи та очікує на оновлення через вебхук...", bot.Self.UserName)

	// telegram.StartEveningReport(bot, sheetsService, cfg) // Якщо функція буде, передаємо cfg

	// ВИПРАВЛЕНО: Передаємо всю структуру cfg замість cfg.SpreadsheetID
	telegram.HandleUpdates(updates, bot, sheetsService, cfg)
}
