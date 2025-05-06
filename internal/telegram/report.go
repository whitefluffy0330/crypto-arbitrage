package telegram

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	sheetsAPI "google.golang.org/api/sheets/v4"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
)

// Надсилає користувачу звіт про прогрес з Google Sheets
func SendProgressReport(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *sheetsAPI.Service, spreadsheetID string) {
	report := sheets.GenerateProgressReport(srv, spreadsheetID)

	message := tgbotapi.NewMessage(msg.Chat.ID, report)
	if _, err := bot.Send(message); err != nil {
		log.Printf("Помилка надсилання звіту: %v", err)
	}
}
