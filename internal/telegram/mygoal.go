package telegram

import (
	"fmt"
	"log"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"
)

func HandleMyGoalCommand(bot *tgbotapi.BotAPI, chatID int64, srv *sheets.Service, spreadsheetID string) {
	readRange := "Цілі!A2:F"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil || len(resp.Values) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ Активної цілі не знайдено.")
		bot.Send(msg)
		return
	}

	for _, row := range resp.Values {
		if len(row) >= 6 && row[3] == "Активна" {
			name := fmt.Sprintf("%v", row[0])
			target := fmt.Sprintf("%v", row[1])
			current := fmt.Sprintf("%v", row[2])
			progress := fmt.Sprintf("%v", row[5])

			msgText := fmt.Sprintf("🎯 Твоя поточна ціль: %s\nНакопичено: $%s із $%s\nПрогрес: %s 🚀", name, current, target, progress)
			msg := tgbotapi.NewMessage(chatID, msgText)
			bot.Send(msg)
			return
		}
	}

	msg := tgbotapi.NewMessage(chatID, "❌ Немає активної цілі в таблиці.")
	bot.Send(msg)
}

func UpdateGoalProgress(srv *sheets.Service, spreadsheetID string) {
	readRange := "Цілі!A2:F"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil || len(resp.Values) == 0 {
		return
	}

	for i, row := range resp.Values {
		if len(row) >= 3 && row[3] == "Активна" {
			target, _ := strconv.Atoi(fmt.Sprintf("%v", row[1]))
			current, _ := strconv.Atoi(fmt.Sprintf("%v", row[2]))
			progress := int(float64(current) / float64(target) * 100)

			_, err := srv.Spreadsheets.Values.Update(spreadsheetID, fmt.Sprintf("Цілі!F%d", i+2), &sheets.ValueRange{
				Values: [][]interface{}{{fmt.Sprintf("%d%%", progress)}},
			}).ValueInputOption("USER_ENTERED").Do()
			if err != nil {
				log.Printf("Помилка оновлення прогресу: %v", err)
			}
			return
		}
	}
}
