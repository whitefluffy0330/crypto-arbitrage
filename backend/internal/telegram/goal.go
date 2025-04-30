package telegram

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"
)

var (
	goalState      string
	tempGoalName   string
	tempGoalAmount string
)

func HandleGoalCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	goalState = "waiting_goal_name"
	msg := tgbotapi.NewMessage(message.Chat.ID, "Введи назву своєї нової цілі:")
	bot.Send(msg)
}

func HandleGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *sheets.Service, spreadsheetID string) {
	switch goalState {
	case "waiting_goal_name":
		tempGoalName = message.Text
		goalState = "waiting_goal_amount"
		msg := tgbotapi.NewMessage(message.Chat.ID, "Яка сума ($) потрібна для цієї цілі?")
		bot.Send(msg)

	case "waiting_goal_amount":
		tempGoalAmount = message.Text
		err := writeGoalRow(srv, spreadsheetID, []interface{}{tempGoalName, tempGoalAmount, 0, "Активна", time.Now().Format("02.01.2006"), "0%"})
		if err != nil {
			log.Printf("Помилка запису цілі: %v", err)
		}
		goalState = ""
		tempGoalName = ""
		tempGoalAmount = ""
		msg := tgbotapi.NewMessage(message.Chat.ID, "🎯 Ціль додано! Тепер працюємо над її досягненням!")
		bot.Send(msg)
	}
}

func writeGoalRow(srv *sheets.Service, spreadsheetID string, values []interface{}) error {
	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, "Цілі!A:F", &sheets.ValueRange{
		Values: [][]interface{}{values},
	}).ValueInputOption("USER_ENTERED").Do()
	return err
}
