// main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var (
	startWorkTime    time.Time
	isWorking        bool
	isBreakRequested bool
	srv              *sheets.Service
	spreadsheetID    string
	bot              *tgbotapi.BotAPI
	chatID           int64

	goalState      string
	tempGoalName   string
	tempGoalAmount string
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Помилка завантаження .env файлу")
	}

	location, _ := time.LoadLocation("Europe/Kyiv")
	time.Local = location

	botToken := os.Getenv("TELEGRAM_TOKEN")
	spreadsheetID = os.Getenv("SPREADSHEET_ID")
	if botToken == "" || spreadsheetID == "" {
		log.Fatal("TELEGRAM_TOKEN або SPREADSHEET_ID не встановлені")
	}

	bot, err = tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	webhookURL := os.Getenv("WEBHOOK_URL")
	_, err = bot.Request(tgbotapi.NewWebhook(webhookURL))
	if err != nil {
		log.Fatal(err)
	}

	creds, _ := os.ReadFile("internal/credentials.json")
	cfg, _ := google.JWTConfigFromJSON(creds, sheets.SpreadsheetsScope)
	client := cfg.Client(context.Background())
	srv, err = sheets.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Google Sheets error: %v", err)
	}

	updates := bot.ListenForWebhook("/webhook")
	go http.ListenAndServeTLS(":443", "/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem", "/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem", nil)

	log.Println("✅ Бот запущено на HTTPS")
	go eveningReport()

	for update := range updates {
		if update.Message != nil {
			chatID = update.Message.Chat.ID
			if update.Message.IsCommand() {
				handleCommand(update.Message)
			} else if goalState != "" {
				handleGoalCreation(update.Message)
			} else {
				handleButton(update.Message)
			}
		} else if update.CallbackQuery != nil {
			handleCallback(update.CallbackQuery)
		}
	}
}

// решта функцій буде додана в наступному блоці коду
