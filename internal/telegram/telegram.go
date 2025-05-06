package telegram

import (
	"log"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"google.golang.org/api/sheets/v4"
)

func InitBot(token string) (*tgbotapi.BotAPI, tgbotapi.UpdatesChannel, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, nil, err
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)
	log.Printf("✅ Telegram бот запущений: @%s", bot.Self.UserName)

	return bot, updates, nil
}

func HandleUpdates(updates tgbotapi.UpdatesChannel, bot *tgbotapi.BotAPI, srv *sheets.Service, spreadsheetID string) {
	for update := range updates {
		HandleUpdate(update, bot, srv, spreadsheetID)
	}
}
