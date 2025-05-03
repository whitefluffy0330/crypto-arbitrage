package telegram

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

func StartWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	// TODO: Реалізувати логіку старту роботи
}

func StopWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	// TODO: Реалізувати логіку зупинки роботи
}

func DayOff(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv any, spreadsheetID string) {
	// TODO: Реалізувати логіку дня відпочинку
}

func ShowMainKeyboard(bot *tgbotapi.BotAPI, cfg any, chatID int64) {
	// TODO: Відправити головну клавіатуру
}

func HandleButtons(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv any, spreadsheetID string) {
	// TODO: Обробка кнопок
}

func HandleCallback(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	// TODO: Обробка callback-даних
}
