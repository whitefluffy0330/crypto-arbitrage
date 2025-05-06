package telegram

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
)

// HandleUpdate обробляє вхідні повідомлення та callback-и
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *sheets.Service, spreadsheetID string) {
	if update.Message != nil {
		switch update.Message.Text {
		case "/start":
			commands.StartWork(bot, update.Message)
		case "/stop":
			commands.StopWork(bot, update.Message)
		case "/dayoff":
			commands.DayOff(bot, update.Message, srv, spreadsheetID)
		case "/goal":
			goal.HandleMyGoalCommand(bot, update.Message.Chat.ID)
		case "/motivation":
			motivationText := motivation.GetRandomMotivation()
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, motivationText)
			bot.Send(msg)
		case "/report":
			ReportProgress(bot, update.Message, srv, spreadsheetID)
		default:
			keyboard.ShowMainKeyboard(bot, update.Message.Chat.ID)
		}
	}

	if update.CallbackQuery != nil {
		goal.HandleCallback(bot, update.CallbackQuery)
	}
}
