package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken      string
	SpreadsheetID string
	ChatID        int64
	WebhookURL    string
}

func LoadEnv() Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Файл .env не знайдено, використовуються системні змінні")
	}

	chatID, err := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)
	if err != nil {
		log.Println("Помилка парсингу TELEGRAM_CHAT_ID")
		chatID = 0
	}

	return Config{
		BotToken:      os.Getenv("TELEGRAM_TOKEN"),
		SpreadsheetID: os.Getenv("SPREADSHEET_ID"),
		WebhookURL:    os.Getenv("WEBHOOK_URL"),
		ChatID:        chatID,
	}
}
