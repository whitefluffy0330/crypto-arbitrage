package telegram

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	sheetsAPI "google.golang.org/api/sheets/v4"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
)

// ReportProgress генерує звіт про дохід і надсилає його користувачу
func ReportProgress(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *sheetsAPI.Service, spreadsheetID string) {
	reportText := sheets.GenerateProgressReport(srv, spreadsheetID)

	response := tgbotapi.NewMessage(msg.Chat.ID, reportText)
	if _, err := bot.Send(response); err != nil {
		log.Printf("Помилка надсилання звіту: %v", err)
	}
}
