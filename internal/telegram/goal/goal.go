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
// Тепер приймає cfg config.Config.
// Специфічні callback-и для закриття цілі обробляються в handler.go.
func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, srv *gsheets.Service, cfg config.Config) { // <<< Оновлена сигнатура
	chatID := callback.Message.Chat.ID
	callbackData := callback.Data
	userName := callback.From.UserName

	log.Printf("Підпакет goal: HandleCallback отримав дані: '%s' від [%s] (ChatID: %d)", callbackData, userName, chatID)

	// Видалено блок switch, оскільки він посилався на невизначені константи,
	// а основна логіка підтвердження/скасування закриття цілі тепер в handler.go.
	// Ця функція залишається як заглушка для можливих майбутніх callback-ів цілей.

	// Відповідь на CallbackQuery тепер надсилається з handler.go.
}
