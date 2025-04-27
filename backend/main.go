package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

var bot *tgbotapi.BotAPI

func main() {
	// Завантаження змінних середовища
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	if telegramToken == "" {
		log.Fatal("TELEGRAM_TOKEN is not set in environment variables")
	}

	// Ініціалізація бота
	bot, err = tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Налаштування webhook
	webhookURL := "https://vadymnewchapter.pp.ua/webhook"
	_, err = bot.Request(tgbotapi.NewWebhook(webhookURL))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Webhook set to:", webhookURL)

	// Fiber сервер
	app := fiber.New()

	// Обробка POST-запитів від Telegram на /webhook
	app.Post("/webhook", func(c *fiber.Ctx) error {
		update := tgbotapi.Update{}
		if err := json.Unmarshal(c.Body(), &update); err != nil {
			return c.SendStatus(http.StatusBadRequest)
		}

		// Логування оновлення
		log.Printf("Received message from chat ID %d", update.Message.Chat.ID)

		if update.Message != nil {
			// Проста відповідь на будь-яке повідомлення
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Отримав твоє повідомлення!")
			bot.Send(msg)
		}

		return c.SendStatus(http.StatusOK)
	})

	// Слухаємо HTTPS
	log.Fatal(app.ListenTLS("0.0.0.0:443",
		"/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem",
		"/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem"))
}
