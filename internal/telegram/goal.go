package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// userGoals зберігає активні цілі користувачів
var userGoals = make(map[int64]string)

// HandleGoalInput обробляє введення користувачем цілі
func HandleGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID
	userGoals[chatID] = message.Text

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🎯 Ціль збережено: %s", message.Text))
	bot.Send(msg)
}

// HandleCallback — шаблон обробки callback-кнопок
func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	// Тут можна реалізувати логіку натискання кнопок
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "🔘 Натиснута кнопка: "+callback.Data)
	bot.Send(msg)
}
