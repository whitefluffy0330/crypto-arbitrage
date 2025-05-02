package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"crypto-arbitrage/internal/config"
	"crypto-arbitrage/internal/sheets"
	"crypto-arbitrage/internal/telegram"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

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

	srv := sheets.InitGoogleSheets()

	updates := bot.ListenForWebhook("/webhook")
	go func() {
		log.Fatal(http.ListenAndServeTLS(":443",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem",
			nil))
	}()

	telegram.StartEveningReport(bot, srv, cfg.SpreadsheetID, cfg.ChatID)
	telegram.HandleUpdates(updates, bot, srv, cfg)
}
