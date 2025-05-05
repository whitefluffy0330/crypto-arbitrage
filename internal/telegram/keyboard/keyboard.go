package keyboard

import (
	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ShowMainKeyboard показує головну клавіатуру користувачу
func ShowMainKeyboard(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🎯 Моя ціль"),
			tgbotapi.NewKeyboardButton("🔁 Старт"),
			tgbotapi.NewKeyboardButton("⛔️ Стоп"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🏖 Вихідний"),
			tgbotapi.NewKeyboardButton("📊 Прогрес"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Оберіть опцію з меню:")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}
