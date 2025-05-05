package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleCloseGoalInput обробляє повідомлення про завершення цілі
func HandleCloseGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID
	userInput := message.Text

	// Тут можна реалізувати логіку для збереження завершеної цілі, якщо потрібно
	response := fmt.Sprintf("✅ Ціль '%s' успішно закрита!", userInput)

	msg := tgbotapi.NewMessage(chatID, response)
	bot.Send(msg)
}
