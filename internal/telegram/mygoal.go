package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleMyGoalCommand(bot *tgbotapi.BotAPI, chatID int64) {
	messageText := fmt.Sprintf("🎯 Поточна ціль поки не задана.\n\nСкористайся командою /goal щоб встановити свою ціль.")
	msg := tgbotapi.NewMessage(chatID, messageText)
	bot.Send(msg)
}
