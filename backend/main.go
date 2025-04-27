package main

import (
	"log"
	"os"
	"time"
	"strings"

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
	sheetName     string
)

func main() {
	// Завантаження змінних середовища
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Помилка завантаження .env файлу")
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	spreadsheetID = os.Getenv("SPREADSHEET_ID")
	sheetName = os.Getenv("SHEET_NAME")

	if botToken == "" || chatID == "" || spreadsheetID == "" || sheetName == "" {
		log.Fatal("Одне або кілька середовищних змінних не встановлено")
	}

	// Підключення до Telegram
	bot, err := tgbotapi.NewBotAPI(botToken)
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

	// Налаштування Telegram обробника
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

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

			writeRow([]interface{}{
				startWorkTime.Format("02.01.2006 15:04"),
				endWorkTime.Format("02.01.2006 15:04"),
				durationStr,
				"Робочий",
			})

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Роботу завершено та записано в Google Sheets!")
			bot.Send(msg)

			// Обнуляємо старт
			startWorkTime = time.Time{}

		case "Вихідний день":
			writeRow([]interface{}{
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

func writeRow(values []interface{}) {
	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheetName+"!A:D", &sheets.ValueRange{
		Values: [][]interface{}{values},
	}).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка запису в Google Sheets: %v", err)
	}
}
