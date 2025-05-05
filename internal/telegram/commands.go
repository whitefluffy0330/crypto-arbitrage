package commands

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
)

func StartWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	text := "✅ Робочий день розпочато. Успішної роботи!"
	message := tgbotapi.NewMessage(msg.Chat.ID, text)
	if _, err := bot.Send(message); err != nil {
		log.Printf("Помилка при відправці повідомлення StartWork: %v", err)
	}
}

func StopWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	text := "🛑 Робочий день завершено. Гарного відпочинку!"
	message := tgbotapi.NewMessage(msg.Chat.ID, text)
	if _, err := bot.Send(message); err != nil {
		log.Printf("Помилка при відправці повідомлення StopWork: %v", err)
	}
}

func DayOff(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, _ any, _ string) {
	text := "📅 Сьогодні вихідний день."
	message := tgbotapi.NewMessage(msg.Chat.ID, text)
	if _, err := bot.Send(message); err != nil {
		log.Printf("Помилка при відправці повідомлення DayOff: %v", err)
	}
}
