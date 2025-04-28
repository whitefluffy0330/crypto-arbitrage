package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var (
	startWorkTime    time.Time
	srv              *sheets.Service
	spreadsheetID    string
	bot              *tgbotapi.BotAPI
	chatID           int64
	isWorking        bool
	isBreakRequested bool
	goalState        string
	tempGoalName     string
	tempGoalAmount   string
	waitingProfit    bool
)

const (
	workSheet   = "Робочі сесії"
	goalSheet   = "Цілі"
	incomeSheet = "Мапа доходу"
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

	webhookURL := "https://vadymnewchapter.pp.ua/webhook"
	wh, _ := tgbotapi.NewWebhook(webhookURL)
	_, err = bot.Request(wh)
	if err != nil {
		log.Fatal(err)
	}

	credsData, err := os.ReadFile("internal/credentials.json")
	if err != nil {
		log.Fatalf("Не знайдено файл credentials.json: %v", err)
	}
	config, err := google.JWTConfigFromJSON(credsData, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Помилка створення конфігурації: %v", err)
	}
	client := config.Client(context.Background())
	srv, err = sheets.NewService(context.Background(), option.WithHTTPClient(client))
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

	go morningReport()

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
			if waitingProfit {
				handleProfitEntry(update.Message)
				continue
			}
			handleButton(update.Message)
		}
		if update.CallbackQuery != nil {
			handleCallback(update.CallbackQuery)
		}
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
	case "mygoal":
		goalState = "waiting_goal_name"
		msg := tgbotapi.NewMessage(message.Chat.ID, "Введи назву своєї нової цілі:")
		bot.Send(msg)
	default:
		showMainKeyboard(message.Chat.ID)
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
	isWorking = true
	isBreakRequested = false
	go startBreakTimer(message.Chat.ID)
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
	durationStr := strings.TrimSpace(fmt.Sprintf("%d год %d хв", hours, minutes))
	writeRow(workSheet, []interface{}{startWorkTime.Format("02.01.2006 15:04"), endWorkTime.Format("02.01.2006 15:04"), durationStr, "Робочий"})
	isWorking = false
	isBreakRequested = false
	startWorkTime = time.Time{}
	msg := tgbotapi.NewMessage(message.Chat.ID, "Роботу завершено! Вкажи скільки $ ти сьогодні заробив:")
	bot.Send(msg)
	waitingProfit = true
	go profitTimeoutChecker(message.Chat.ID)
}

func registerDayOff(message *tgbotapi.Message) {
	writeRow(workSheet, []interface{}{"", "", "", "Вихідний"})
	msg := tgbotapi.NewMessage(message.Chat.ID, "Вихідний день записано!")
	bot.Send(msg)
}
func handleProfitEntry(message *tgbotapi.Message) {
	waitingProfit = false
	profit, err := strconv.ParseFloat(message.Text, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Будь ласка, введи суму прибутку у числовому форматі (наприклад 150).")
		bot.Send(msg)
		waitingProfit = true
		return
	}
	updateIncome(profit)
}

func updateIncome(profit float64) {
	readRange := incomeSheet + "!A:D"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil || len(resp.Values) < 2 {
		log.Printf("Помилка читання доходів: %v", err)
		return
	}

	for idx, row := range resp.Values {
		if len(row) >= 2 && row[0] == "Арбітраж" {
			currentStr := fmt.Sprintf("%v", row[2])
			targetStr := fmt.Sprintf("%v", row[3])

			current, _ := strconv.ParseFloat(currentStr, 64)
			target, _ := strconv.ParseFloat(targetStr, 64)

			newCurrent := current + profit
			newPercent := (newCurrent / target) * 100

			updateRange := fmt.Sprintf("%s!C%d:D%d", incomeSheet, idx+1, idx+1)
			_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, &sheets.ValueRange{
				Values: [][]interface{}{
					{newCurrent, fmt.Sprintf("%.2f%%", newPercent)},
				},
			}).ValueInputOption("USER_ENTERED").Do()
			if err != nil {
				log.Printf("Помилка оновлення доходу: %v", err)
			}

			motivations := []string{
				"Ти на шляху до перемоги! 🚀",
				"Ще один крок ближче до нової реальності! ✨",
				"Твої зусилля працюють! 🔥",
				"Ти твориш новий етап життя! 💪",
			}
			text := fmt.Sprintf("+%.2f$ зароблено сьогодні! %s\nПрогрес по цілі: %.2f%% ✅", profit, motivations[rand.Intn(len(motivations))], newPercent)
			bot.Send(tgbotapi.NewMessage(chatID, text))
			return
		}
	}
}

func profitTimeoutChecker(chatID int64) {
	time.Sleep(15 * time.Minute)
	if waitingProfit {
		bot.Send(tgbotapi.NewMessage(chatID, "Не забудь ввести скільки $ ти заробив сьогодні!"))
	}
	time.Sleep(15 * time.Minute)
	if waitingProfit {
		waitingProfit = false
		updateIncome(0)
		bot.Send(tgbotapi.NewMessage(chatID, "Час вичерпано. Записано 0$ на сьогодні."))
	}
}

func handleGoalCreation(message *tgbotapi.Message) {
	switch goalState {
	case "waiting_goal_name":
		tempGoalName = message.Text
		goalState = "waiting_goal_amount"
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Введи суму цілі ($):"))
	case "waiting_goal_amount":
		tempGoalAmount = message.Text
		if _, err := strconv.ParseFloat(tempGoalAmount, 64); err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Будь ласка, введи суму в числовому форматі."))
			return
		}
		writeRow(goalSheet, []interface{}{tempGoalName, tempGoalAmount})
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Ціль додано успішно! 🚀"))
		goalState = ""
	}
}

func writeRow(sheetName string, values []interface{}) {
	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheetName+"!A:D", &sheets.ValueRange{Values: [][]interface{}{values}}).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка запису в Google Sheets: %v", err)
	}
}

func showMainKeyboard(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Оберіть дію:")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Почати роботу"),
			tgbotapi.NewKeyboardButton("Закінчити роботу"),
			tgbotapi.NewKeyboardButton("Вихідний день")),
	)
	bot.Send(msg)
}

func startBreakTimer(chatID int64) {
	for isWorking {
		time.Sleep(90 * time.Minute)
		if isWorking && !isBreakRequested {
			sendBreakReminder(chatID)
		}
	}
}

func sendBreakReminder(chatID int64) {
	breakMessages := []string{
		"Час трохи розім'ятись! 🚶‍♂️",
		"Перерва — найкращий заряд енергії! ⚡",
		"Не забувай: найкращі ідеї приходять на прогулянці! 🌳",
	}
	text := breakMessages[rand.Intn(len(breakMessages))]
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ок, йду відпочивати", "start_break"),
			tgbotapi.NewInlineKeyboardButtonData("Я вже тут", "end_break")),
	)
	bot.Send(msg)
}

func handleCallback(query *tgbotapi.CallbackQuery) {
	switch query.Data {
	case "start_break":
		if !isBreakRequested {
			isBreakRequested = true
			bot.Send(tgbotapi.NewMessage(query.Message.Chat.ID, "Відпочивай! ☕"))
		}
	case "end_break":
		if isBreakRequested {
			isBreakRequested = false
			bot.Send(tgbotapi.NewMessage(query.Message.Chat.ID, "Чудово! Повертаємось до роботи! 🚀"))
		} else {
			bot.Send(tgbotapi.NewMessage(query.Message.Chat.ID, "Спочатку натисни \"Ок, йду відпочивати\"!"))
		}
	}
	bot.Request(tgbotapi.NewCallback(query.ID, ""))
}

func morningReport() {
	for {
		now := time.Now()
		nextReport := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, now.Location())
		if now.After(nextReport) {
			nextReport = nextReport.Add(24 * time.Hour)
		}
		time.Sleep(nextReport.Sub(now))
		if chatID != 0 {
			readRange := workSheet + "!A:D"
			resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
			if err != nil {
				log.Printf("Помилка читання Google Sheets: %v", err)
				continue
			}
			if len(resp.Values) < 2 {
				continue
			}
			lastRow := resp.Values[len(resp.Values)-2]
			var reportText string
			if len(lastRow) >= 4 {
				if lastRow[3] == "Вихідний" {
					reportText = "Учора був вихідний день. Відпочинок — теж успіх! 🔥"
				} else {
					reportText = "Учора ти пропрацював: " + lastRow[2].(string) + ". Чудова робота! 💪"
				}
			} else {
				reportText = "Немає даних за вчорашній день."
			}
			msg := tgbotapi.NewMessage(chatID, reportText)
			        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(query.ID, ""))
    }
}

func handleGoalCreation(message *tgbotapi.Message) {
    // Тимчасова заглушка для створення цілі
}
