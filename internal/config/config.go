package config

import (
	"os"
	"strconv"
)

type Config struct {
	BotToken      string
	SpreadsheetID string
	ChatID        int64
}

func LoadEnv() Config {
	chatID, _ := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)

	return Config{
		BotToken:      os.Getenv("TELEGRAM_TOKEN"),
		SpreadsheetID: os.Getenv("SPREADSHEET_ID"),
		ChatID:        chatID,
	}
}
