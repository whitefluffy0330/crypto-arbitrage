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
	bot                 *tgbotapi.BotAPI
	srv                 *sheets.Service
	spreadsheetID       string
	chatID              int64
	startWorkTime       time.Time
	isWorking           bool
	isBreakRequested    bool
	breakDuration       = 90 * time.Minute
	goalCreationStarted bool
	tempGoalName        string
	tempGoalAmount      string
)

func main() {
	err := godotenv.Load("backend/.env")
	if err != nil {
		log.Fatal("Помилка завантаження .env файлу")
	}

	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	spreadsheetID = os.Getenv("SPREADSHEET_ID")
	webhookURL := os.Getenv("WEBHOOK_URL")
	chatIDRaw := os.Getenv("TELEGRAM_CHAT_ID")

	if telegramToken == "" || spreadsheetID == "" || webhookURL == "" || chatIDRaw == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN, SPREADSHEET_ID, TELEGRAM_CHAT_ID або WEBHOOK_URL не встановлені")
	}

	chatID, err = strconv.ParseInt(chatIDRaw, 10, 64)
	if err != nil {
		log.Fatal("Помилка конвертації chat ID:", err)
	}

	bot, err = tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	creds, err := os.ReadFile("backend/internal/credentials.json")
	if err != nil {
		log.Fatalf("Не знайдено credentials.json: %v", err)
	}

	config, err := google.JWTConfigFromJSON(creds, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Помилка авторизації Google: %v", err)
	}

	client := config.Client(ctx)
	srv, err = sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Не вдалося створити сервіс Google Sheets: %v", err)
	}

	go morningReport()
	go breakReminder()

	_, err = bot.Request(tgbotapi.NewWebhook(webhookURL))
	if err != nil {
		log.Fatalf("Помилка встановлення webhook: %v", err)
	}

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		update := tgbotapi.Update{}
		if err := json.NewDecoder(r.Body).Decode(&update); err == nil {
			if update.Message != nil {
				handleMessage(update.Message)
			} else if update.CallbackQuery != nil {
				handleCallback(update.CallbackQuery)
			}
		}
	})

	log.Println("Бот запущено та слухає HTTPS!")

	err = http.ListenAndServeTLS(":443",
		"/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem",
		"/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem", nil)
	if err != nil {
		log.Fatalf("HTTPS сервер завершив роботу з помилкою: %v", err)
	}
}

func handleMessage(msg *tgbotapi.Message) {
	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			sendStartKeyboard(msg.Chat.ID)
		case "mygoal":
			goalCreationStarted = true
			tempGoalName = ""
			tempGoalAmount = ""
			bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Введіть назву вашої цілі:"))
		default:
			bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Невідома команда"))
		}
		return
	}

	if goalCreationStarted {
		if tempGoalName == "" {
			tempGoalName = msg.Text
			bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Введіть суму у $:"))
		} else if tempGoalAmount == "" {
			tempGoalAmount = msg.Text
			values := []interface{}{tempGoalName, tempGoalAmount}
			writeRow("Мапа доходу", values)
			goalCreationStarted = false
			bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Ціль збережено!"))
		}
		return
	}

	switch msg.Text {
	case "Почати роботу":
		if !isWorking {
			startWorkTime = time.Now()
			isWorking = true
			isBreakRequested = false
			bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Роботу розпочато."))
		}
	case "Закінчити роботу":
		if isWorking {
			duration := time.Since(startWorkTime)
			isWorking = false
			writeRow("Робочі сесії", []interface{}{startWorkTime.Format("02.01.2006 15:04"), time.Now().Format("15:04"), duration.String()})
			bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Роботу завершено. Пропрацьовано "+duration.String()))
		}
	case "Вихідний день":
		writeRow("Робочі сесії", []interface{}{time.Now().Format("02.01.2006"), "-", "-", "Вихідний"})
		isWorking = false
		bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Вихідний зафіксовано."))
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

func handleCallback(callback *tgbotapi.CallbackQuery) {
	// реалізуємо пізніше
}

func writeRow(sheet string, values []interface{}) {
	ctx := context.Background()
	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheet, &sheets.ValueRange{
		Values: [][]interface{}{values},
	}).ValueInputOption("RAW").Context(ctx).Do()
	if err != nil {
		log.Printf("Помилка запису в таблицю: %v", err)
	}
}

func morningReport() {
	for {
		now := time.Now().In(time.FixedZone("Europe/Kyiv", 2*60*60))
		if now.Hour() == 8 && now.Minute() == 0 {
			requiredMonthlyIncome := 2000.0
			daysInMonth := 30
			readRange := "Робочі сесії!A:D"
			resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
			if err != nil {
				log.Printf("Помилка читання таблиці: %v", err)
				continue
			}
			daysWorked := 0
			for _, row := range resp.Values {
				if len(row) >= 4 && row[3] != "Вихідний" {
					daysWorked++
				}
			}
			remainingDays := daysInMonth - daysWorked
			if remainingDays <= 0 {
				remainingDays = 1
			}
			dailyTarget := int(requiredMonthlyIncome / float64(remainingDays))
			msg := tgbotapi.NewMessage(chatID, "Щоденний звіт:
Днів до кінця місяця: "+strconv.Itoa(remainingDays)+"
Ціль: "+strconv.Itoa(dailyTarget)+"$")
			bot.Send(msg)
			time.Sleep(time.Minute)
		}
		time.Sleep(30 * time.Second)
	}
}

func breakReminder() {
	for {
		if isWorking && !isBreakRequested {
			if time.Since(startWorkTime) >= breakDuration {
				isBreakRequested = true
				msg := tgbotapi.NewMessage(chatID, "Час на перерву! Перепочиньте трохи.")
				bot.Send(msg)
			}
		}
		time.Sleep(time.Minute)
	}
}
