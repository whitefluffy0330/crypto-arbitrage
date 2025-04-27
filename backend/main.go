package main

import (
	"log"
	"os"
	"time"
	"strings"
	"strconv"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var (
	startWorkTime time.Time
	srv           *sheets.Service
	spreadsheetID string
)

func main() {
	// Завантаження .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Помилка завантаження .env файлу")
	}

	botToken := os.Getenv("TELEGRAM_TOKEN")
	spreadsheetID = os.Getenv("SPREADSHEET_ID")

	if botToken == "" || spreadsheetID == "" {
		log.Fatal("TELEGRAM_TOKEN або SPREADSHEET_ID не встановлені")
	}

	// Підключення до Telegram
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	// Встановлюємо webhook
	webhookURL := "https://vadymnewchapter.pp.ua/webhook"
	_, err = bot.Request(tgbotapi.NewWebhook(webhookURL))
	if err != nil {
		log.Fatal(err)
	}

	// Підключення до Google Sheets
	credsData, err := os.ReadFile("internal/credentials.json")
	if err != nil {
		log.Fatalf("Не знайдено файл credentials.json: %v", err)
	}
	config, err := google.JWTConfigFromJSON(credsData, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Помилка створення конфігурації: %v", err)
	}
	client := config.Client(oauth2.NoContext)
	srv, err = sheets.NewService(oauth2.NoContext, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Помилка підключення до Google Sheets: %v", err)
	}

	updates := bot.ListenForWebhook("/webhook")
	go func() {
		log.Fatal(http.ListenAndServeTLS(":443", "/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem", "/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem", nil))
	}()

	log.Println("Бот запущено!")

	for update := range updates {
		if update.Message == nil {
			continue
		}

		switch update.Message.Text {
		case "Почати роботу":
			startWorkTime = time.Now()
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Роботу розпочато!")
			bot.Send(msg)

		case "Закінчити роботу":
			if startWorkTime.IsZero() {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Спочатку потрібно натиснути «Почати роботу»!")
				bot.Send(msg)
				continue
			}

			endWorkTime := time.Now()
			duration := endWorkTime.Sub(startWorkTime)
			hours := int(duration.Hours())
			minutes := int(duration.Minutes()) % 60
			durationStr := strings.TrimSpace(
				strings.Join([]string{
					func() string {
						if hours > 0 {
							return strconv.Itoa(hours) + " год"
						}
						return ""
					}(),
					func() string {
						if minutes > 0 {
							return strconv.Itoa(minutes) + " хв"
						}
						return ""
					}(),
				}, " "),
			)

			writeRow("Робочі сесії", []interface{}{
				startWorkTime.Format("02.01.2006 15:04"),
				endWorkTime.Format("02.01.2006 15:04"),
				durationStr,
				"Робочий",
			})

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Роботу завершено та записано в Google Sheets!")
			bot.Send(msg)

			startWorkTime = time.Time{}

		case "Вихідний день":
			writeRow("Робочі сесії", []interface{}{
				"", "", "", "Вихідний",
			})

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Вихідний день записано!")
			bot.Send(msg)

		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Будь ласка, використовуйте кнопки!")
			bot.Send(msg)
		}
	}
}

func writeRow(sheetName string, values []interface{}) {
	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheetName+"!A:D", &sheets.ValueRange{
		Values: [][]interface{}{values},
	}).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка запису в Google Sheets: %v", err)
	}
}
