package telegram

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
)

// HandleProgressReportCommand обробляє команду /progress
func HandleProgressReportCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *sheets.Service, spreadsheetID string) {
	report, err := sheets.GenerateProgressReport(srv, spreadsheetID, message.Chat.ID)
	if err != nil {
		log.Printf("❌ Помилка генерації звіту: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Не вдалося згенерувати звіт 😢")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, report)
	bot.Send(msg)
}
