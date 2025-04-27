package telegram

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
)

type TelegramMessage struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

func SendTelegramMessage(message string) error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	msg := TelegramMessage{
		ChatID: chatID,
		Text:   message,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	tgURL := "https://api.telegram.org/bot" + botToken + "/sendMessage"
	_, err = http.Post(tgURL, "application/json", bytes.NewBuffer(msgBytes))
	return err
}
