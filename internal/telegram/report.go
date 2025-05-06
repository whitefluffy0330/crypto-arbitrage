package telegram

import (
	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"google.golang.org/api/sheets/v4"
)

func ReportProgress(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *sheets.Service, spreadsheetID string) {
	reportText := "Поки що звіт недоступний (функція в розробці)"

	response := tgbotapi.NewMessage(msg.Chat.ID, reportText)
	bot.Send(response)
}
