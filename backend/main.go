package main

import (
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, proceeding anyway.")
	}

	botToken := os.Getenv("TELEGRAM_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_TOKEN is not set in environment variables")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	app := fiber.New()

	app.Post("/webhook", func(c *fiber.Ctx) error {
		update := tgbotapi.Update{}
		if err := c.BodyParser(&update); err != nil {
			return err
		}

		if update.Message != nil {
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "start":
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привіт! Обери дію 👇")
					msg.ReplyMarkup = mainKeyboard()
					bot.Send(msg)
				}
			} else if update.Message.Text != "" {
				switch update.Message.Text {
				case "Почати роботу":
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "🚀 Почали роботу!"))
				case "Завершити роботу":
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "✅ Роботу завершено. Молодець!"))
				case "Вихідний день":
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "🌴 Відпочиваємо сьогодні!"))
				default:
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Я поки тебе не розумію, спробуй натиснути кнопку!"))
				}
			}
		}
		return c.SendStatus(200)
	})

	// Налаштування webhook
	webhookURL := os.Getenv("WEBHOOK_URL")
	wh, err := tgbotapi.NewWebhook(webhookURL + "/webhook")
	if err != nil {
		log.Fatal(err)
	}

	_, err = bot.Request(wh)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(app.Listen(":8080"))
}

func mainKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Почати роботу"),
			tgbotapi.NewKeyboardButton("Завершити роботу"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Вихідний день"),
		),
	)
}
