package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
)

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *sheets.Service, spreadsheetID string) {
	if update.Message != nil {
		switch update.Message.Text {
		case "Почати роботу":
			commands.StartWork(bot, update.Message)
		case "Завершити роботу":
			commands.StopWork(bot, update.Message)
		case "Взяти вихідний":
			commands.DayOff(bot, update.Message, srv, spreadsheetID)
		case "Моя ціль":
			goal.HandleMyGoalCommand(bot, update.Message.Chat.ID, srv, spreadsheetID)
		case "Мотивація":
			motivation.SendRandomMotivation(bot, update.Message.Chat.ID)
		default:
			keyboard.ShowMainKeyboard(bot, update.Message.Chat.ID)
		}
	}

	if update.CallbackQuery != nil {
		goal.HandleCallback(bot, update.CallbackQuery)
	}
}
