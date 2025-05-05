package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleGoalInput обробляє введену користувачем ціль
func HandleGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID
	goal := message.Text

	response := fmt.Sprintf("🎯 Твоя нова ціль: \"%s\" записана!", goal)
	msg := tgbotapi.NewMessage(chatID, response)
	bot.Send(msg)
}
