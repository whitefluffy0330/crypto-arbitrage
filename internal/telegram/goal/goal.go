package goal

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleMyGoalCommand(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🧠 Надішли свою ціль у форматі: 1200 грн, 15 днів")
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка при відправці HandleMyGoalCommand: %v", err)
	}
}

func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "✅ Callback опрацьовано.")
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка при обробці callback: %v", err)
	}
}
