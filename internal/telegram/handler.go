package telegram

import (
	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	"google.golang.org/api/sheets/v4"
)

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *sheets.Service, spreadsheetID string) {
	if update.Message != nil {
		switch update.Message.Text {
		case "Старт":
			commands.StartWork(bot, update.Message)
		case "Завершити":
			commands.StopWork(bot, update.Message)
		case "Вихідний":
			commands.DayOff(bot, update.Message, srv, spreadsheetID)
		case "Моя ціль":
			goal.HandleMyGoalCommand(bot, update.Message.Chat.ID, srv, spreadsheetID)
		case "Звіт":
			ReportProgress(bot, update.Message, srv, spreadsheetID)
		default:
			motivationText := motivation.GetRandomMotivation()
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, motivationText)
			bot.Send(msg)

			keyboard.ShowMainKeyboard(bot, update.Message.Chat.ID)
		}
	}

	if update.CallbackQuery != nil {
		goal.HandleCallback(bot, update.CallbackQuery)
	}
}
