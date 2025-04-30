// main.go (фінальна версія з підтримкою HTTPS, Google Sheets, Telegram Bot, цілей, вечірнього звіту, мотивації)

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
	sheetsapi "google.golang.org/api/sheets/v4"

	"backend/internal/telegram"
)

// 🔻 Основні глобальні змінні
var (
	startWorkTime    time.Time
	isWorking        bool
	isBreakRequested bool
	goalState        string
	tempGoalName     string
	tempGoalAmount   string
	srv              *sheetsapi.Service
	spreadsheetID    string
	bot              *tgbotapi.BotAPI
	chatID           int64
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Помилка завантаження .env файлу")
	}

	location, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Fatal("Не вдалося завантажити часову зону Europe/Kyiv")
	}
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
	wh := tgbotapi.NewWebhook(webhookURL)
	if _, err := bot.Request(wh); err != nil {
		log.Fatal(err)
	}

	credsData, err := os.ReadFile("internal/credentials.json")
	if err != nil {
		log.Fatalf("Не знайдено файл credentials.json: %v", err)
	}

	config, err := google.JWTConfigFromJSON(credsData, sheetsapi.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Помилка створення конфігурації: %v", err)
	}
	client := config.Client(context.Background())
	srv, err = sheetsapi.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Помилка підключення до Google Sheets: %v", err)
	}

	updates := bot.ListenForWebhook("/webhook")
	go func() {
		log.Fatal(http.ListenAndServeTLS(":443",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem",
			nil))
	}()

	log.Println("Бот запущено та слухає HTTPS!")

	// ⬇️ Вставка нового модуля звіту
	go telegram.StartEveningReport(bot, srv, spreadsheetID, chatID)

	for update := range updates {
		if update.Message != nil {
			chatID = update.Message.Chat.ID
			if update.Message.IsCommand() {
				handleCommand(update.Message)
				continue
			}
			if goalState != "" {
				handleGoalCreation(update.Message)
				continue
			}
			handleButton(update.Message)
		}
		if update.CallbackQuery != nil {
			handleCallback(update.CallbackQuery)
		}
	}
}

// 🔻 Далі йде залишений функціонал на підтримку старого стилю (тимчасово, до повного винесення в internal/*)
