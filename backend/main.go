package main

import (
	"log"
	"net/http"
	"os"
	"time"
	"strings"
	"strconv"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var (
	startWorkTime time.Time
	srv           *sheets.Service
	spreadsheetID string
	bot           *tgbotapi.BotAPI
)

func main() {
	// Завантаження .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Помилка завантаження .env файлу")
	}

	// Встановлення часового поясу на Київ
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

	// Підключення до Telegram
	bot, err = tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	// Встановлюємо webhook
	webhookURL := "https://vadymnewchapter.pp.ua/webhook"
	wh, _ := tgbotapi.NewWebhook(webhookURL)
	_, err = bot.Request(wh)
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
		log.Fatal(http.ListenAndServeTLS(":443",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem",
			nil))
	}()

	log.Println("Бот запущено та слухає HTTPS!")

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			handleCommand(update.Message)
			continue
		}

		handleButton(update.Message)
	}
}

func handleCommand(message *tgbotapi.Message) {
	switch message.Command() {
	case "почати_роботу":
		startWork(message)
	case "завершити_роботу":
		finishWork(message)
	case "вихідний":
		registerDayOff(message)
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "Команда не знайдена. Використовуйте кнопки.")
		bot.Send(msg)
	}
}

func handleButton(message *tgbotapi.Message) {
	switch message.Text {
	case "Почати роботу":
		startWork(message)
	case "Закінчити роботу":
		finishWork(message)
	case "Вихідний день":
		registerDayOff(message)
	default:
		showMainKeyboard(message.Chat.ID)
	}
}

func startWork(message *tgbotapi.Message) {
	startWorkTime = time.Now()
	msg := tgbotapi.NewMessage(message.Chat.ID, "Роботу розпочато!")
	bot.Send(msg)
}

func finishWork(message *tgbotapi.Message) {
	if startWorkTime.IsZero() {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Спочатку потрібно натиснути «Почати роботу»!")
		bot.Send(msg)
		return
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

	msg := tgbotapi.NewMessage(message.Chat.ID, "Роботу завершено та записано в Google Sheets!")
	bot.Send(msg)

	startWorkTime = time.Time{}
}

func registerDayOff(message *tgbotapi.Message) {
	writeRow("Робочі сесії", []interface{}{
		"", "", "", "Вихідний",
	})

	msg := tgbotapi.NewMessage(message.Chat.ID, "Вихідний день записано!")
	bot.Send(msg)
}

func showMainKeyboard(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Оберіть дію:")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Почати роботу"),
			tgbotapi.NewKeyboardButton("Закінчити роботу"),
			tgbotapi.NewKeyboardButton("Вихідний день"),
		),
	)
	bot.Send(msg)
}

func writeRow(sheetName string, values []interface{}) {
	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheetName+"!A:D", &sheets.ValueRange{
		Values: [][]interface{}{values},
	}).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка запису в Google Sheets: %v", err)
	}
}
