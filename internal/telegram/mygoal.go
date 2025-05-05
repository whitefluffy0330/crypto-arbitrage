package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleMyGoalCommand обробляє команду /mygoal і надсилає повідомлення з поточною ціллю
func HandleMyGoalCommand(bot *tgbotapi.BotAPI, chatID int64) {
	// Тут має бути логіка для отримання цілі з БД або кешу (тимчасово — заглушка)
	currentGoal := "Заробити $2000 цього місяця 💸"
	response := fmt.Sprintf("🎯 Поточна ціль: %s", currentGoal)

	msg := tgbotapi.NewMessage(chatID, response)
	bot.Send(msg)
}
