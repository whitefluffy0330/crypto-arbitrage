package telegram

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
)

func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *sheets.Service, cfg config.Config) {
	for update := range updates {
		if update.Message != nil {
			switch update.Message.Text {
			case "/start_work":
				StartWork(bot, update.Message)
			case "/stop_work":
				StopWork(bot, update.Message)
			case "/dayoff":
				DayOff(bot, update.Message)
			case "/mygoal":
				HandleMyGoalCommand(bot, update.Message, srv, cfg.SpreadsheetID)
			default:
				ShowMainKeyboard(bot, update.Message.Chat.ID)
			}
		} else if update.CallbackQuery != nil {
			HandleCallback(bot, update.CallbackQuery, srv, cfg.SpreadsheetID)
		} else if goal.GoalClosedRecently() && update.Message != nil {
			HandleCloseGoalInput(bot, update.Message, srv, cfg.SpreadsheetID)
		} else if update.Message != nil {
			HandleGoalInput(bot, update.Message, srv, cfg.SpreadsheetID)
		}
	}
}

func StartWork(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	msg := tgbotapi.NewMessage(message.Chat.ID, "Починаємо роботу! 🔥")
	bot.Send(msg)
}

func StopWork(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	msg := tgbotapi.NewMessage(message.Chat.ID, "Робочий день завершено. 💤")
	bot.Send(msg)
}

func DayOff(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	msg := tgbotapi.NewMessage(message.Chat.ID, "Вихідний день. Відпочиваємо! 🎉")
	bot.Send(msg)
}

func ShowMainKeyboard(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/start_work"),
			tgbotapi.NewKeyboardButton("/stop_work"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/dayoff"),
			tgbotapi.NewKeyboardButton("/mygoal"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, "Обери дію:")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}
