package telegram

import (
	"github.com/whitefluffy0330/crypto-arbitrage-bot/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage-bot/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage-bot/internal/telegram/goal"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *sheets.Service, cfg *config.Config) {
	for update := range updates {
		if update.Message != nil {
			cfg.ChatID = update.Message.Chat.ID

			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "start_work":
					StartWork(bot, update.Message)
				case "stop_work":
					StopWork(bot, update.Message, srv, cfg.SpreadsheetID)
				case "day_off":
					DayOff(bot, update.Message, srv, cfg.SpreadsheetID)
				case "mygoal":
					HandleMyGoalCommand(bot, cfg.ChatID, srv, cfg.SpreadsheetID)
				case "closegoal":
					HandleCloseGoalCommand(bot, cfg.ChatID)
				default:
					ShowMainKeyboard(bot, cfg.ChatID)
				}
				continue
			}

			HandleGoalInput(bot, update.Message, srv, cfg.SpreadsheetID)
			HandleCloseGoalInput(bot, cfg.ChatID, srv, cfg.SpreadsheetID, update.Message.Text)
			HandleButtons(bot, update.Message, srv, cfg.SpreadsheetID)

			if goal.GoalClosedRecently() {
				HandleGoalCommand(bot, update.Message)
			}
		}

		if update.CallbackQuery != nil {
			HandleCallback(bot, update.CallbackQuery)
		}
	}
}
