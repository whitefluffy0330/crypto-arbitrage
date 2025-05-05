package telegram

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
)

func HandleUpdates(bot *tgbotapi.BotAPI, updates tgbotapi.UpdatesChannel, srv *sheets.Service, spreadsheetID string) {
	for update := range updates {
		if update.Message != nil {
			switch update.Message.Text {
			case "🔁 Старт":
				commands.StartWork(bot, update.Message)
			case "⛔️ Стоп":
				commands.StopWork(bot, update.Message)
			case "🏖 Вихідний":
				commands.DayOff(bot, update.Message, srv, spreadsheetID)
			case "🎯 Моя ціль":
				goal.HandleMyGoalCommand(bot, update.Message.Chat.ID, srv, spreadsheetID)
			case "📊 Прогрес":
				ReportProgress(bot, update.Message.Chat.ID, srv, spreadsheetID)
			default:
				keyboard.ShowMainKeyboard(bot, update.Message.Chat.ID)
			}
		}

		if update.CallbackQuery != nil {
			goal.HandleCallback(bot, update.CallbackQuery)
		}
	}
}
