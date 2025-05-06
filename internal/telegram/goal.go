package telegram

import (
	"fmt"
	"log" // Додано для логування помилок

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ВАЖЛИВО: Оголошення 'var userGoals = make(map[int64]string)' ВИДАЛЕНО ЗВІДСИ.
// Тепер воно коректно визначене лише в файлі internal/telegram/telegram.go
// разом зі змінною userGoalsMutex для безпечного доступу.

// HandleGoalInput обробляє введення користувачем тексту цілі.
// Ця функція призначена для обробки повідомлення, яке користувач надсилає
// ПІСЛЯ того, як отримав запит від команди /goal (з підпакета goal).
//
// TODO: Поточна проблема: handler.go не знає, що наступне повідомлення користувача
// після команди /goal потрібно передати саме сюди. Для цього потрібно буде
// реалізувати механізм управління станом діалогу (наприклад, зберігати,
// що користувач chatID зараз у стані "очікування введення цілі").
// Ми повернемося до цього на етапі реалізації логіки.
func HandleGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID
	goalTextFromUser := message.Text // Текст, введений користувачем

	// TODO: Тут має бути логіка парсингу тексту 'goalTextFromUser'
	// наприклад, "1200 грн, 15 днів", щоб отримати суму, валюту, термін тощо.
	// Наразі ми просто зберігаємо весь введений текст як ціль.

	// Використовуємо userGoals та userGoalsMutex, оголошені в telegram.go (в цьому ж пакеті)
	userGoalsMutex.Lock() // Блокуємо м'ютекс перед записом до мапи
	userGoals[chatID] = goalTextFromUser
	userGoalsMutex.Unlock() // Розблоковуємо м'ютекс після запису

	confirmationText := fmt.Sprintf("🎯 Вашу ціль \"%s\" попередньо збережено! (потрібна реалізація парсингу)", goalTextFromUser)
	msg := tgbotapi.NewMessage(chatID, confirmationText)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка надсилання підтвердження цілі для чату %d: %v", chatID, err)
	}
}

/*
// Нижче наведена функція HandleCallback, яка була у вашому файлі.
// Вона наразі не використовується, оскільки ваш handler.go
// для обробки callback-запитів викликає функцію HandleCallback
// з ПІДПАКЕТА "goal" (тобто з internal/telegram/goal/goal.go).

// Щоб уникнути плутанини та потенційних конфліктів імен, цю функцію
// краще закоментувати або видалити, якщо вона не має іншого призначення.
// Якщо вона вам потрібна для чогось іншого, її слід перейменувати.

func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	// Тут можна реалізувати логіку натискання кнопок
	responseText := "🔘 Натиснута кнопка (з telegram/goal.go - НЕ ВИКОРИСТОВУЄТЬСЯ): " + callback.Data
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, responseText)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка надсилання відповіді на callback (з telegram/goal.go) для чату %d: %v", callback.Message.Chat.ID, err)
	}

	// Також не забувайте відповідати на сам CallbackQuery
	// callbackResp := tgbotapi.NewCallback(callback.ID, "Оброблено "+callback.Data)
	// if _, err := bot.AnswerCallbackQuery(callbackResp); err != nil {
	// 	log.Printf("Помилка відповіді на CallbackQuery: %v", err)
	// }
}
*/
