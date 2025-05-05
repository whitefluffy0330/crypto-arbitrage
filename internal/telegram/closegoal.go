package telegram

import (
	"log"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"
)

// HandleCloseGoalInput завершує активну ціль
func HandleCloseGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *sheets.Service, spreadsheetID string) {
	userID := strconv.FormatInt(message.Chat.ID, 10)

	// Оновлення статусу цілі
	writeRange := userID + "!E2" // Припустимо, що в E2 знаходиться статус
	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{{"завершено"}},
	}
	_, err := srv.Spreadsheets.Values.Update(spreadsheetID, writeRange, valueRange).ValueInputOption("RAW").Do()
	if err != nil {
		log.Printf("❌ Не вдалося оновити статус цілі: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Помилка при завершенні цілі.")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "✅ Ціль завершено! Гарна робота 💪")
	bot.Send(msg)
}
