package telegram

import (
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"
)

// HandleGoalInput обробляє введення цілі
func HandleGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *sheets.Service, spreadsheetID string) {
	userID := strconv.FormatInt(message.Chat.ID, 10)
	goalText := strings.TrimSpace(message.Text)

	writeRange := userID + "!A2"
	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{{goalText}},
	}

	_, err := srv.Spreadsheets.Values.Update(spreadsheetID, writeRange, valueRange).ValueInputOption("RAW").Do()
	if err != nil {
		log.Printf("❌ Не вдалося зберегти ціль: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Помилка при збереженні цілі.")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "✅ Ціль збережена! Тепер працюй над нею кожного дня!")
	bot.Send(msg)
}

// HandleCallback обробляє callback-запити (наприклад, від inline-кнопок)
func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Обробка callback: "+callback.Data)
	bot.Send(msg)
}
