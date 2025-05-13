package keyboard

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
)

// Константи для текстів кнопок
const (
	BtnMyGoal              = "🎯 Моя ціль"
	BtnProgress            = "📊 Прогрес"
	BtnFundingRates        = "💹 Funding Rates"
	BtnWorkStart           = "🔁 Старт"
	BtnWorkStop            = "⛔️ Стоп"
	BtnWorkDayOff          = "🏖 Вихідний"
	BtnCloseGoal           = "❌ Закрити ціль"
	BtnAddInvestment       = "➕ Додати Інвестицію"
	BtnSetFundingThreshold = "⚙️ Поріг Funding"
	BtnSpreads             = "📈 Спреди" // Нова кнопка для спредів
)

// ShowMainKeyboard показує головну клавіатуру користувачу
func ShowMainKeyboard(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnMyGoal),
			tgbotapi.NewKeyboardButton(BtnProgress),
			tgbotapi.NewKeyboardButton(BtnFundingRates),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnWorkStart),
			tgbotapi.NewKeyboardButton(BtnWorkStop),
			tgbotapi.NewKeyboardButton(BtnWorkDayOff),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnCloseGoal),
			tgbotapi.NewKeyboardButton(BtnAddInvestment),
			tgbotapi.NewKeyboardButton(BtnSetFundingThreshold),
		),
		tgbotapi.NewKeyboardButtonRow( // Новий рядок для кнопки Спредів
			tgbotapi.NewKeyboardButton(BtnSpreads),
		),
	)
	keyboard.ResizeKeyboard = true

	msg := tgbotapi.NewMessage(chatID, "Головне меню:")
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
