package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"

	internalSheets "github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
)

// HandleReportCommand надсилає звіт у відповідь на команду
func HandleReportCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *sheets.Service, spreadsheetID string) {
	chatID := message.Chat.ID
	report := internalSheets.GenerateProgressReport(srv, spreadsheetID)
	msg := tgbotapi.NewMessage(chatID, report)
	bot.Send(msg)
}
