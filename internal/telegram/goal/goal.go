package goal

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Додаємо імпорт gsheets для типу srv у сигнатурі HandleCallback
	gsheets "google.golang.org/api/sheets/v4"
)

// HandleMyGoalCommand надсилає користувачеві запит на введення цілі.
// Ця функція залишається без змін.
func HandleMyGoalCommand(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🧠 Надішли свою ціль у форматі: 1200 грн, 15 днів")
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка при відправці HandleMyGoalCommand (підпакет goal): %v", err)
	}
}

// HandleCallback обробляє callback-запити, пов'язані з цілями.
// Тепер приймає srv та spreadsheetID, хоча наразі їх не використовує активно,
// оскільки специфічні callback-и для закриття цілі обробляються в handler.go.
// Цю функцію можна розширити в майбутньому для іншої логіки inline-кнопок, пов'язаних з цілями.
func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, srv *gsheets.Service, spreadsheetID string) {
	chatID := callback.Message.Chat.ID
	callbackData := callback.Data

	log.Printf("Підпакет goal: HandleCallback отримав дані: '%s' для ChatID: %d. MessageID: %d", callbackData, chatID, callback.Message.MessageID)

	// Наразі основна логіка для 'confirm_close_goal' та 'cancel_close_goal' знаходиться в handler.go (пакет telegram).
	// Ця функція може обробляти інші специфічні для "цілей" callback-и, якщо вони з'являться.
	// Наприклад, можна надіслати якесь загальне повідомлення або нічого не робити,
	// якщо callback вже був оброблений (хоча відповідь на callbackQuery все одно потрібна).

	// Приклад: якщо це якийсь інший callback, не оброблений у handler.go
	// responseText := fmt.Sprintf("Отримано callback '%s' у модулі цілей.", callbackData)
	// msg := tgbotapi.NewMessage(chatID, responseText)
	// if _, err := bot.Send(msg); err != nil {
	// 	log.Printf("Помилка надсилання відповіді з goal.HandleCallback: %v", err)
	// }

	// Відповідь на CallbackQuery (щоб прибрати "годинник") тепер обробляється в handler.go
	// після виклику цієї функції або після обробки відомих callbackData.
	// Якщо ця функція буде самостійно надсилати повідомлення у відповідь на callback,
	// вона також повинна викликати bot.Request(tgbotapi.NewCallback(...))
}
