package goal

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	gsheets "google.golang.org/api/sheets/v4"
)

// HandleMyGoalCommand надсилає користувачеві запит на введення суми цілі.
func HandleMyGoalCommand(bot *tgbotapi.BotAPI, chatID int64) {
	// Змінено текст: тепер питаємо лише суму (валюта опціональна)
	msg := tgbotapi.NewMessage(chatID, "🎯 Введіть суму вашої цілі на поточний місяць (наприклад, `15000 ГРН` або `500 USD`). Валюта опціональна (за замовчуванням UAH).")
	msg.ParseMode = tgbotapi.ModeMarkdown // Додаємо Markdown для форматування
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка при відправці HandleMyGoalCommand (підпакет goal): %v", err)
	}
}

// HandleCallback обробляє callback-запити, пов'язані з цілями.
func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, srv *gsheets.Service, cfg config.Config) { 
	chatID := callback.Message.Chat.ID
	callbackData := callback.Data
	userName := callback.From.UserName
	log.Printf("Підпакет goal: HandleCallback отримав дані: '%s' від [%s] (ChatID: %d)", callbackData, userName, chatID)
	// Залишається заглушкою для майбутніх callback-ів
}

// Константи, які використовувалися в handler.go
// const (
//	 CallbackConfirmCloseGoal = "confirm_close_goal"
//	 CallbackCancelCloseGoal  = "cancel_close_goal"
// )
