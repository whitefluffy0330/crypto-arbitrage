package telegram

import (
	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"log"
)

func HandleUpdate(update tgbotapi.Update, bot *tgbotapi.BotAPI, srv *sheets.Service, spreadsheetID string) {
	if update.Message != nil {
		msg := update.Message

		switch msg.Text {
		case "/start":
			welcome := "Привіт! Надішли свою ціль або обери дію з меню ⬇️"
			keyboard.ShowMainKeyboard(bot, msg.Chat.ID, welcome)

		case "Я працюю 💼":
			commands.StartWork(bot, msg)

		case "Я завершив роботу 📤":
			commands.StopWork(bot, msg)

		case "Сьогодні вихідний 🧘‍♂️":
			commands.DayOff(bot, msg, srv, spreadsheetID)

		case "Моя ціль 🎯":
			goal.HandleMyGoalCommand(bot, msg.Chat.ID)

		case "Звіт за день 📊":
			ReportProgress(bot, msg, srv, spreadsheetID)

		default:
			motiv := motivation.GetRandomMotivation()
			keyboard.ShowMainKeyboard(bot, msg.Chat.ID, motiv)
		}
	}

	if update.CallbackQuery != nil {
		goal.HandleCallback(bot, update.CallbackQuery)
	}
}
