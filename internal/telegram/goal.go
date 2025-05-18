package telegram // Залишаємо в тому ж пакеті telegram

import (
	"log"
	// "fmt" // Не потрібен, якщо HandleGoalInput тут не формує відповідь
	// "regexp" // Не потрібен, якщо парсинг в HandleGoalInput в handler.go
	// "strconv" // Не потрібен
	// "strings" // Не потрібен
	// "time"    // Не потрібен

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config" // Потрібен для HandleCallback
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // sheets.KyivLocation тепер в telegram.KyivLocation
	gsheets "google.golang.org/api/sheets/v4" // Потрібен для HandleCallback
)

// Функція HandleGoalInput (з вашої відповіді #66) ТЕПЕР ПЕРЕМІЩЕНА ДО handler.go
// або має бути викликана з handler.go, якщо логіка парсингу залишається тут.
// Для уникнення redeclared, припускаємо, що HandleGoalInput буде в handler.go

// HandleMyGoalCommand надсилає користувачеві запит на введення суми цілі.
// Цю функцію можна залишити тут або перенести в handler.go
func HandleMyGoalCommand(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🎯 Введіть суму вашої цілі на поточний місяць (наприклад, `15000 ГРН` або `500 USD`). Валюта опціональна (за замовчуванням UAH).")
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка при відправці HandleMyGoalCommand (пакет telegram, файл goal.go): %v", err)
	}
}

// HandleCallback обробляє callback-запити, пов'язані з цілями (з вашого goal.go).
func HandleGoalCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, srv *gsheets.Service, cfg config.Config) { 
	// Ця функція у вас називалася HandleCallback, перейменовую на HandleGoalCallback,
	// щоб уникнути конфлікту з HandleCallback у handler.go, якщо такий є.
	// Або їх потрібно об'єднати в handler.go.
	chatID := callback.Message.Chat.ID
	callbackData := callback.Data
	userName := callback.From.UserName
	log.Printf("Пакет telegram (файл goal.go): HandleGoalCallback отримав дані: '%s' від [%s] (ChatID: %d)", callbackData, userName, chatID)
	// Тут має бути ваша логіка обробки callback-запитів, специфічних для цілей,
	// наприклад, підтвердження закриття цілі, якщо ця логіка не в handler.go.
	// Якщо CallbackConfirmCloseGoal та CallbackCancelCloseGoal обробляються в handler.go,
	// то ця функція може бути не потрібна або має обробляти інші callback'и.
	
	// Приклад відповіді на callback, щоб Telegram не показував "годинник"
	answerCallbackCfg := tgbotapi.NewCallback(callback.ID, "Обробка запиту цілі...")
 	if _, err := bot.Request(answerCallbackCfg); err != nil {
 		log.Printf("Помилка відповіді на callback в HandleGoalCallback: %v", err)
 	}
}
