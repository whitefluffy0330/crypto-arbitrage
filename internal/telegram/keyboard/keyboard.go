package keyboard

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
)

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
	BtnSpreads             = "📈 Спреди"
)

// Константи для CallbackData кнопок вибору біржі для фандингу
const (
	CallbackFundingBinance = "funding_binance"
	CallbackFundingBybit   = "funding_bybit"
	CallbackFundingOKX     = "funding_okx"
	CallbackFundingMEXC    = "funding_mexc"
	// Додамо сюди інші біржі, коли вони будуть реалізовані
)

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
		tgbotapi.NewKeyboardButtonRow(
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

func CreateConfirmationKeyboard(yesCallbackData, noCallbackData string) tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Так", yesCallbackData),
			tgbotapi.NewInlineKeyboardButtonData("🚫 Ні", noCallbackData),
		),
	)
	return keyboard
}

// CreateFundingExchangeSelectionKeyboard створює inline-клавіатуру для вибору біржі для фандингу
func CreateFundingExchangeSelectionKeyboard() tgbotapi.InlineKeyboardMarkup {
	// Список підтримуваних бірж (назви для кнопок та їх callback-дані)
	// В майбутньому це можна буде генерувати динамічно на основі конфігурації
	exchanges := []struct {
		Name         string
		CallbackData string
	}{
		{"Binance", CallbackFundingBinance},
		{"Bybit", CallbackFundingBybit},
		{"OKX", CallbackFundingOKX},
		{"MEXC", CallbackFundingMEXC},
		// Додайте сюди інші біржі, коли вони будуть готові
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	var currentRow []tgbotapi.InlineKeyboardButton

	for i, ex := range exchanges {
		currentRow = append(currentRow, tgbotapi.NewInlineKeyboardButtonData(ex.Name, ex.CallbackData))
		// Робимо по 2 кнопки в рядку для кращого вигляду
		if (i+1)%2 == 0 || i == len(exchanges)-1 {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(currentRow...))
			currentRow = []tgbotapi.InlineKeyboardButton{} // Очищаємо для наступного рядка
		}
	}
	// Можна додати кнопку "Скасувати" або "Всі біржі" (з обережністю)
	// rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Скасувати", "funding_cancel")))


	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}
