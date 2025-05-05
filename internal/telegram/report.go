package telegram

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"google.golang.org/api/sheets/v4"
)

// HandleProgressReport надсилає звіт користувачу
func HandleProgressReport(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *sheets.Service, spreadsheetID string) {
	userID := fmt.Sprintf("%d", message.Chat.ID)
	reportText, err := sheets.GenerateProgressReport(srv, spreadsheetID, userID)
	if err != nil {
		log.Printf("❌ Помилка при генерації звіту: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Не вдалося згенерувати звіт.")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, reportText)
	bot.Send(msg)
}
