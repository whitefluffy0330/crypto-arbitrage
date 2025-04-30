package main

import (
	"log"
	"net/http"
	"os"

	"backend/internal/config"
	"backend/internal/sheets"
	"backend/internal/telegram"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"google.golang.org/api/sheets/v4"
)

func main() {
	_ = godotenv.Load()

	botToken := os.Getenv("TELEGRAM_TOKEN")
	spreadsheetID := os.Getenv("SPREADSHEET_ID")
	chatID := config.GetChatID()

	if botToken == "" || spreadsheetID == "" || chatID == 0 {
		log.Fatal("Не задані обов'язкові змінні середовища")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
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

	telegram.StartEveningReport(bot, srv, spreadsheetID, chatID)

	for update := range updates {
		if update.Message != nil {
			chatID = update.Message.Chat.ID

			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "start_work":
					telegram.StartWork(bot, update.Message)
				case "stop_work":
					telegram.StopWork(bot, update.Message, srv, spreadsheetID)
				case "day_off":
					telegram.DayOff(bot, update.Message, srv, spreadsheetID)
				case "mygoal":
					telegram.HandleMyGoalCommand(bot, chatID, srv, spreadsheetID)
				case "closegoal":
					telegram.HandleCloseGoalCommand(bot, chatID)
				default:
					telegram.ShowMainKeyboard(bot, chatID)
				}
				continue
			}

			telegram.HandleGoalInput(bot, update.Message, srv, spreadsheetID)
			telegram.HandleCloseGoalInput(bot, chatID, srv, spreadsheetID, update.Message.Text)
			telegram.HandleButtons(bot, update.Message, srv, spreadsheetID)
		}

		if update.CallbackQuery != nil {
			telegram.HandleCallback(bot, update.CallbackQuery)
		}
	}
}
