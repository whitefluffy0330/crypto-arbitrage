package goal

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Додаємо імпорт config для типу config.Config у сигнатурі HandleCallback
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
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
// Тепер приймає cfg config.Config, хоча наразі її не використовує,
// оскільки специфічні callback-и обробляються в handler.go.
// Може бути розширена в майбутньому.
func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, srv *gsheets.Service, cfg config.Config) { // <<< ЗМІНЕНО СИГНАТУРУ
	chatID := callback.Message.Chat.ID
	callbackData := callback.Data
	userName := callback.From.UserName

	log.Printf("Підпакет goal: HandleCallback отримав дані: '%s' від [%s] (ChatID: %d)", callbackData, userName, chatID)

	// Наразі специфічна логіка для підтвердження/скасування закриття цілі
	// знаходиться в handler.go (пакет telegram), оскільки вона вимагає
	// доступу до функції DeleteUserGoal та кешу userGoals з того пакета.

	// Ця функція може бути розширена для обробки інших inline-кнопок,
	// що стосуються цілей (наприклад, редагування цілі, перегляд історії тощо),
	// якщо ви додасте такий функціонал.

	// Наприклад, можна додати логіку для невідомих callback-ів, що сюди потрапили:
	switch callbackData {
	case CallbackConfirmCloseGoal, CallbackCancelCloseGoal:
		// Ці обробляються в handler.go, тут нічого не робимо
		log.Printf("Підпакет goal: Callback '%s' оброблено в handler.go", callbackData)
	default:
		log.Printf("Підпакет goal: Отримано невідомий callback data '%s'. Поки що ігнорується.", callbackData)
		// Можна надіслати повідомлення користувачеві або просто проігнорувати.
		// Відповідь на CallbackQuery (щоб прибрати "годинник") все одно буде надіслано з handler.go.
	}
}

// Константи для callback даних (можливо, їх варто винести в спільне місце?)
// Ці константи вже визначені в handler.go, тут вони для ясності логіки switch
// const (
// 	CallbackConfirmCloseGoal = "confirm_close_goal"
// 	CallbackCancelCloseGoal  = "cancel_close_goal"
// )
