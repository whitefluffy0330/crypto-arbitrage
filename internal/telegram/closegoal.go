package telegram

import (
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"
)

// HandleCloseGoalInput обробляє завершення цілі
func HandleCloseGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *sheets.Service, spreadsheetID string) {
	userID := fmt.Sprintf("%d", message.Chat.ID)
	finalText := strings.TrimSpace(message.Text)
	now := time.Now().Format("02.01.2006")

	writeRange := userID + "!A3:B3"
	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{{now, finalText}},
	}

	_, err := srv.Spreadsheets.Values.Update(spreadsheetID, writeRange, valueRange).ValueInputOption("RAW").Do()
	if err != nil {
		log.Printf("❌ Не вдалося завершити ціль: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Помилка при завершенні цілі.")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "🎉 Ціль успішно завершено! Ти молодець!")
	bot.Send(msg)
}
