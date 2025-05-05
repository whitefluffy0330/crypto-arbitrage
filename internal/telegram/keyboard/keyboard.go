package keyboard

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func MainKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🚀 Почати робочий день"),
			tgbotapi.NewKeyboardButton("🛑 Завершити робочий день"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📅 Вихідний"),
			tgbotapi.NewKeyboardButton("🎯 Моя ціль"),
		),
	)
}
