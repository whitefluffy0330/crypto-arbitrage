package keyboard

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log" 
)

// ... (Ваша функція ShowMainKeyboard залишається тут) ...
func ShowMainKeyboard(bot *tgbotapi.BotAPI, chatID int64) {
	// ... (код ShowMainKeyboard) ...
}


// НОВА ФУНКЦІЯ: CreateConfirmationKeyboard створює inline-клавіатуру "Так/Ні"
func CreateConfirmationKeyboard(yesCallbackData, noCallbackData string) tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Так", yesCallbackData), // Кнопка "Так" з даними для callback
			tgbotapi.NewInlineKeyboardButtonData("🚫 Ні", noCallbackData),   // Кнопка "Ні" з даними для callback
		),
	)
	return keyboard
}
