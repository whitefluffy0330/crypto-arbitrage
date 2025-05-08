package keyboard

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
)

// ShowMainKeyboard показує головну клавіатуру користувачу
func ShowMainKeyboard(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🎯 Моя ціль"),
			tgbotapi.NewKeyboardButton("📊 Прогрес"),
			tgbotapi.NewKeyboardButton("💹 Funding Rates"), // <<< НОВА КНОПКА
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🔁 Старт"),
			tgbotapi.NewKeyboardButton("⛔️ Стоп"),
			tgbotapi.NewKeyboardButton("🏖 Вихідний"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("❌ Закрити ціль"),
			tgbotapi.NewKeyboardButton("/add_investment"), // Поки що команда, можна зробити кнопкою
		),
	)
	keyboard.ResizeKeyboard = true 

	msg := tgbotapi.NewMessage(chatID, "Оберіть опцію з меню:")
	msg.ReplyMarkup = keyboard
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка надсилання головної клавіатури для чату %d: %v", chatID, err)
	}
}

// CreateConfirmationKeyboard створює inline-клавіатуру "Так/Ні"
func CreateConfirmationKeyboard(yesCallbackData, noCallbackData string) tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Так", yesCallbackData),
			tgbotapi.NewInlineKeyboardButtonData("🚫 Ні", noCallbackData),
		),
	)
	return keyboard
}
