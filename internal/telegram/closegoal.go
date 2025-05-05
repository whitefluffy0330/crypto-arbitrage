package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CloseUserGoal очищає ціль користувача
func CloseUserGoal(bot *tgbotapi.BotAPI, chatID int64) {
	// Видаляємо ціль з пам’яті
	userGoals[chatID] = ""

	msg := tgbotapi.NewMessage(chatID, "✅ Ціль завершено. Готовий рухатись далі!")
	bot.Send(msg)
}
