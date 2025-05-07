package telegram

import (
	"log" // Додамо логування

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CloseUserGoal видаляє ціль користувача з пам'яті.
// TODO: Потрібно визначити, яка команда/кнопка буде викликати цю функцію з handler.go.
func CloseUserGoal(bot *tgbotapi.BotAPI, chatID int64) {
	// Використовуємо функцію DeleteUserGoal, визначену в telegram.go (в цьому ж пакеті),
	// яка безпечно видаляє запис з мапи userGoals та логує дію.
	DeleteUserGoal(chatID)

	msgText := "✅ Вашу поточну ціль було видалено. Ви можете встановити нову за допомогою команди /goal."
	msg := tgbotapi.NewMessage(chatID, msgText)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка надсилання повідомлення про закриття цілі для чату %d: %v", chatID, err)
	}
}
