package main

import (
	"context"
	"log"
	"net/http"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // Для sheets.SpreadsheetsScope
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation" // Для InitMotivationSeed

	// Ось цей імпорт, з яким виникала помилка
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

	// ДІАГНОСТИЧНИЙ РЯДОК: Явне використання пакета tgbotapi
	var _ tgbotapi.Update // Цей рядок додано для явної вказівки компілятору, що пакет використовується

	cfg := config.LoadEnv()

	if cfg.BotToken == "" || cfg.SpreadsheetID == "" {
		log.Fatal("Критична помилка: Не задані обов'язкові змінні середовища TELEGRAM_TOKEN та SPREADSHEET_ID")
	}

	bot, err := telegram.InitBot(cfg.BotToken) // telegram.InitBot повертає *tgbotapi.BotAPI
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}

	log.Printf("Бот @%s ініціалізовано.", bot.Self.UserName)

	webhookBaseURL := "https://vadymnewchapter.pp.ua"
	webhookPath := "/webhook_" + bot.Token
	certFilePath := "" // Залиште порожнім для Let's Encrypt з надійним CA

	err = telegram.SetWebhook(bot, webhookBaseURL, webhookPath, certFilePath)
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

	updates := bot.ListenForWebhook(webhookPath) // bot.ListenForWebhook повертає tgbotapi.UpdatesChannel

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

	log.Printf("Бот @%s готовий до роботи та очікує на оновлення через вебхук...", bot.Self.UserName)

	// telegram.StartEveningReport(bot, sheetsService, cfg.SpreadsheetID, cfg.ChatID) // Закоментовано, поки не реалізовано

	telegram.HandleUpdates(updates, bot, sheetsService, cfg.SpreadsheetID)
}
