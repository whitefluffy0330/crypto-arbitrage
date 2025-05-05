package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleMyGoalCommand відправляє користувачу його активну ціль
func HandleMyGoalCommand(bot *tgbotapi.BotAPI, chatID int64) {
	goal, ok := userGoals[chatID]
	if !ok || goal == "" {
		msg := tgbotapi.NewMessage(chatID, "❌ У вас ще немає встановленої цілі.")
		bot.Send(msg)
		return
	}

	text := fmt.Sprintf("🎯 Ваша поточна ціль: %s", goal)
	msg := tgbotapi.NewMessage(chatID, text)
	bot.Send(msg)
}
