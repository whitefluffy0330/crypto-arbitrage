package keyboard

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log" // Додамо логування для Send
)

// ShowMainKeyboard показує головну клавіатуру користувачу
func ShowMainKeyboard(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🎯 Моя ціль"),
			tgbotapi.NewKeyboardButton("📊 Прогрес"), // Перемістив для логічного групування
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🔁 Старт"),
			tgbotapi.NewKeyboardButton("⛔️ Стоп"),
			tgbotapi.NewKeyboardButton("🏖 Вихідний"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("❌ Закрити ціль"), // Нова кнопка
		),
	)
	// keyboard.ResizeKeyboard = true // Можна додати, щоб клавіатура підлаштовувалася під розмір

	msg := tgbotapi.NewMessage(chatID, "Оберіть опцію з меню:")
	msg.ReplyMarkup = keyboard
	if _, err := bot.Send(msg); err != nil { // Додано перевірку помилки
		log.Printf("Помилка надсилання головної клавіатури для чату %d: %v", chatID, err)
	}
}
