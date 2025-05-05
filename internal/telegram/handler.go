package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"

	internalSheets "github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
)

// ReportProgress генерує та надсилає звіт користувачу
func ReportProgress(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *sheets.Service, spreadsheetID string) {
	chatID := message.Chat.ID

	report := internalSheets.GenerateProgressReport(srv, spreadsheetID)
	bot.Send(tgbotapi.NewMessage(chatID, report))
}
