package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

func appContext() context.Context {
	return context.Background()
}

func main() {
	cfg := config.LoadEnv()

	if cfg.BotToken == "" || cfg.SpreadsheetID == "" || cfg.ChatID == 0 {
		log.Fatal("Не задані обов'язкові змінні середовища")
	}

	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatal(err)
	}

	err = telegram.SetWebhook(bot)
	if err != nil {
		log.Fatal(err)
	}

	ctx := appContext()
	credentials, err := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Помилка авторизації Google Sheets: %v", err)
	}

	sheetsService, err := sheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err)
	}

	updates := bot.ListenForWebhook("/webhook")
	go func() {
		log.Fatal(http.ListenAndServeTLS(":443",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem",
			nil))
	}()

	telegram.StartEveningReport(bot, sheetsService, cfg.SpreadsheetID, cfg.ChatID)
	telegram.HandleUpdates(updates, bot, sheetsService, cfg)
}
