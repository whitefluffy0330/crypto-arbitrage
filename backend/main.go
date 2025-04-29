
package main

import (
	"context"
	"encoding/json"
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
	bot               *tgbotapi.BotAPI
	srv               *sheets.Service
	spreadsheetID     string
	chatID            int64
	startWorkTime     time.Time
	isWorking         bool
	isBreakRequested  bool
	breakDuration     = 90 * time.Minute
	goalCreationStage int
	tempGoalName      string
	tempGoalAmount    string
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
		log.Fatal("TELEGRAM_BOT_TOKEN, SPREADSHEET_ID, TELEGRAM_CHAT_ID або WEBHOOK_URL не встановлені")
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

	go morningReport()
	go breakReminder()

	updates := bot.ListenForWebhook("/webhook")
	_, err = bot.SetWebhook(tgbotapi.NewWebhook(webhookURL))
	if err != nil {
		log.Fatal("Помилка встановлення webhook")
	}

	log.Println("Бот запущено та слухає HTTPS!")

	go func() {
		err := http.ListenAndServeTLS(":443",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem", nil)
		if err != nil {
			log.Fatalf("Помилка HTTPS сервера: %v", err)
		}
	}()

	for update := range updates {
		if update.Message != nil {
			handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			handleCallback(update.CallbackQuery)
		}
	}
}

func handleMessage(message *tgbotapi.Message) {
	if goalCreationStage > 0 {
		handleGoalCreation(message)
		return
	}

	if message.IsCommand() {
		switch message.Command() {
		case "start":
			sendStartKeyboard(message.Chat.ID)
		case "mygoal":
			goalCreationStage = 1
			msg := tgbotapi.NewMessage(message.Chat.ID, "Введіть назву цілі:")
			bot.Send(msg)
		default:
			msg := tgbotapi.NewMessage(message.Chat.ID, "Невідома команда")
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
			writeRow("Робочі сесії", []interface{}{
				startWorkTime.Format("02.01.2006 15:04"),
				time.Now().Format("02.01.2006 15:04"),
				duration.String(),
			})
			isWorking = false
			msg := tgbotapi.NewMessage(message.Chat.ID, "Роботу завершено! Тривалість: "+duration.String())
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

func handleGoalCreation(message *tgbotapi.Message) {
	switch goalCreationStage {
	case 1:
		tempGoalName = message.Text
		goalCreationStage = 2
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Введіть суму цілі у $:"))
	case 2:
		tempGoalAmount = message.Text
		values := []interface{}{tempGoalName, tempGoalAmount}
		writeRow("Мапа доходу", values)
		tempGoalName, tempGoalAmount = "", ""
		goalCreationStage = 0
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Ціль успішно додано!"))
	}
}

func handleCallback(callback *tgbotapi.CallbackQuery) {
	switch callback.Data {
	case "break_start":
		isBreakRequested = true
		bot.Send(tgbotapi.NewMessage(callback.Message.Chat.ID, "Добре! Зроби собі чай і відпочинь трохи!"))
	case "break_end":
		isBreakRequested = false
		bot.Send(tgbotapi.NewMessage(callback.Message.Chat.ID, "Круто! Повертаємось до роботи."))
	}
}

func morningReport() {
	for {
		now := time.Now().In(time.FixedZone("Kyiv", 2*60*60))
		if now.Hour() == 8 && now.Minute() == 0 {
			writeRow("Робочі сесії", []interface{}{time.Now().Format("02.01.2006"), "Звіт", "-", "Автоматично"})
			bot.Send(tgbotapi.NewMessage(chatID, "Нагадування: новий день — нові можливості!"))
		}
		time.Sleep(1 * time.Minute)
	}
}

func breakReminder() {
	for {
		if isWorking && !isBreakRequested && time.Since(startWorkTime) > breakDuration {
			isBreakRequested = true
			msg := tgbotapi.NewMessage(chatID, "Час зробити перерву!")
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Ок, йду відпочивати", "break_start"),
					tgbotapi.NewInlineKeyboardButtonData("Я вже тут", "break_end"),
				),
			)
			bot.Send(msg)
		}
		time.Sleep(1 * time.Minute)
	}
}

func writeRow(sheetName string, values []interface{}) {
	ctx := context.Background()
	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheetName, &sheets.ValueRange{
		Values: [][]interface{}{values},
	}).ValueInputOption("RAW").Context(ctx).Do()
	if err != nil {
		log.Printf("Помилка запису в Google Sheets: %v", err)
	}
}
