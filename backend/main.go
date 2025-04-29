package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var (
	bot            *tgbotapi.BotAPI
	srv            *sheets.Service
	spreadsheetID  string
	chatID         int64
	startWorkTime  time.Time
	isWorking      bool
	isBreakRequested bool
	breakDuration  = 90 * time.Minute
	goalCreation   bool
	tempGoalName   string
	tempGoalAmount string
)

func main() {
	err := godotenv.Load("backend/.env")
	if err != nil {
		log.Fatal("Помилка завантаження .env файлу")
	}

	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	spreadsheetID = os.Getenv("SPREADSHEET_ID")
	chatIDRaw := os.Getenv("TELEGRAM_CHAT_ID")
	webhookURL := os.Getenv("WEBHOOK_URL")

	if telegramToken == "" || spreadsheetID == "" || chatIDRaw == "" || webhookURL == "" {
		log.Fatal("TELEGRAM_TOKEN, SPREADSHEET_ID, TELEGRAM_CHAT_ID або WEBHOOK_URL не встановлені")
	}

	chatID, err = strconv.ParseInt(chatIDRaw, 10, 64)
	if err != nil {
		log.Fatal("Помилка конвертації chat ID:", err)
	}

	bot, err = tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true

	ctx := context.Background()
	credentials, err := os.ReadFile("backend/internal/credentials.json")
	if err != nil {
		log.Fatalf("Не знайдено файл credentials.json: %v", err)
	}

	config, err := google.JWTConfigFromJSON(credentials, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Помилка авторизації Google Sheets: %v", err)
	}
	client := config.Client(ctx)

	srv, err = sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Не вдалося створити клієнта Google Sheets: %v", err)
	}

	webhookCfg, err := tgbotapi.NewWebhook(webhookURL)
	if err != nil {
		log.Fatal("Помилка створення конфігурації webhook:", err)
	}

	_, err = bot.Request(webhookCfg)
	if err != nil {
		log.Fatal("Помилка встановлення webhook:", err)
	}

	log.Println("Бот запущено та слухає HTTPS!")

	go func() {
		err := http.ListenAndServeTLS(":443", "/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem", "/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem", nil)
		if err != nil {
			log.Fatalf("Помилка HTTPS сервера: %v", err)
		}
	}()

	updates := bot.ListenForWebhook("/webhook")

	for update := range updates {
		if update.Message != nil {
			handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			handleCallback(update.CallbackQuery)
		}
	}
}

func handleMessage(message *tgbotapi.Message) {
	if message.IsCommand() {
		switch message.Command() {
		case "start":
			sendStartKeyboard(message.Chat.ID)
		case "mygoal":
			goalCreation = true
			msg := tgbotapi.NewMessage(message.Chat.ID, "Введіть назву цілі:")
			bot.Send(msg)
		default:
			msg := tgbotapi.NewMessage(message.Chat.ID, "Невідома команда")
			bot.Send(msg)
		}
		return
	}

	if goalCreation {
		if tempGoalName == "" {
			tempGoalName = message.Text
			msg := tgbotapi.NewMessage(message.Chat.ID, "Введіть цільову суму у $:")
			bot.Send(msg)
		} else {
			tempGoalAmount = message.Text
			values := []interface{}{tempGoalName, tempGoalAmount, "Активний", time.Now().Format("02.01.2006")}
			writeRow("Цілі", values)
			valuesIncome := []interface{}{"Арбітраж", "Активний", 0, tempGoalAmount, "0%", time.Now().Format("02.01.2006"), "Працювати над напрямком"}
			writeRow("Мапа доходу", valuesIncome)
			tempGoalName = ""
			tempGoalAmount = ""
			goalCreation = false
			msg := tgbotapi.NewMessage(message.Chat.ID, "Ціль додано!")
			bot.Send(msg)
		}
		return
	}

	switch message.Text {
	case "Почати роботу":
		if !isWorking {
			startWorkTime = time.Now()
			isWorking = true
			isBreakRequested = false
			msg := tgbotapi.NewMessage(message.Chat.ID, "Робоча сесія розпочалася!")
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Ви вже працюєте!")
			bot.Send(msg)
		}
	case "Закінчити роботу":
		if isWorking {
			duration := time.Since(startWorkTime)
			writeRow("Робочі сесії", []interface{}{startWorkTime.Format("02.01.2006 15:04"), time.Now().Format("02.01.2006 15:04"), duration.String(), "Робочий"})
			isWorking = false
			msg := tgbotapi.NewMessage(message.Chat.ID, "Робочу сесію завершено! Пропрацьовано "+duration.String())
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Робоча сесія не активна.")
			bot.Send(msg)
		}
	case "Вихідний день":
		writeRow("Робочі сесії", []interface{}{time.Now().Format("02.01.2006"), "-", "-", "Вихідний"})
		isWorking = false
		msg := tgbotapi.NewMessage(message.Chat.ID, "Вихідний день зафіксовано.")
		bot.Send(msg)
	}
}

func sendStartKeyboard(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Оберіть дію:")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Почати роботу"),
			tgbotapi.NewKeyboardButton("Закінчити роботу"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Вихідний день"),
		),
	)
	bot.Send(msg)
}

func handleCallback(cb *tgbotapi.CallbackQuery) {
	// обробка callback'ів поки не реалізована
}

func writeRow(sheetName string, values []interface{}) {
	ctx := context.Background()
	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheetName, &sheets.ValueRange{
		Values: [][]interface{}{values},
	}).ValueInputOption("RAW").Context(ctx).Do
