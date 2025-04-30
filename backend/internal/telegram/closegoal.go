package telegram

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/whitefluffy0330/crypto-arbitrage-bot/backend/internal/telegram/goal"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"
)

var (
	closingGoalState string
)

func HandleCloseGoalCommand(bot *tgbotapi.BotAPI, chatID int64) {
	closingGoalState = "waiting_final_amount"
	msg := tgbotapi.NewMessage(chatID, "🎯 Вкажи фактичну суму ($), за яку ти купив ціль:")
	bot.Send(msg)
}

func HandleCloseGoalInput(bot *tgbotapi.BotAPI, chatID int64, srv *sheets.Service, spreadsheetID string, text string) {
	if closingGoalState != "waiting_final_amount" {
		return
	}

	readRange := "Цілі!A2:F"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil || len(resp.Values) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "🚫 Помилка при читанні таблиці цілей."))
		return
	}

	amount, err := strconv.Atoi(text)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Введи число без символів, напр. 1850"))
		return
	}

	for i, row := range resp.Values {
		if len(row) >= 4 && row[3] == "Активна" {
			updateRange := fmt.Sprintf("Цілі!C%d:D%d", i+2, i+2)
			values := [][]interface{}{{amount, "Завершено"}}
			_, err := srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, &sheets.ValueRange{
				Values: values,
			}).ValueInputOption("USER_ENTERED").Do()
			if err != nil {
				log.Printf("Помилка оновлення цілі: %v", err)
			}

			goal.MarkGoalClosed()

			msg := fmt.Sprintf("✅ Ціль успішно закрита! Фактична сума: $%d", amount)
			bot.Send(tgbotapi.NewMessage(chatID, msg))
			closingGoalState = ""
			return
		}
	}

	bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Активної цілі не знайдено."))
	closingGoalState = ""
}
