package telegram

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"
)

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *sheets.Service, spreadsheetID string) {
	if update.Message != nil {
		switch update.Message.Text {
		case "🚀 Почати день":
			StartWork(bot, update.Message)
		case "⛔️ Завершити день":
			StopWork(bot, update.Message)
		case "🏖 Вихідний":
			DayOff(bot, update.Message, srv, spreadsheetID)
		case "🎯 Моя ціль":
			HandleMyGoalCommand(bot, update.Message.Chat.ID, srv, spreadsheetID)
		default:
			ShowMainKeyboard(bot, update.Message.Chat.ID, srv, spreadsheetID)
		}
	}

	if update.CallbackQuery != nil {
		HandleCallback(bot, update.CallbackQuery)
	}
}
