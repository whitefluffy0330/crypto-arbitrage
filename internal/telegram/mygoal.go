package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Тут можна зберігати цілі в пам’яті, або підключити базу/таблицю
var userGoals = make(map[int64]string)

// HandleMyGoalCommand надсилає користувачу його активну ціль
func HandleMyGoalCommand(bot *tgbotapi.BotAPI, chatID int64) {
	goal, ok := userGoals[chatID]
	if !ok || goal == "" {
		bot.Send(tgbotapi.NewMessage(chatID, "😕 У тебе ще немає встановленої цілі."))
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🎯 Твоя поточна ціль: %s", goal))
	bot.Send(msg)
}
