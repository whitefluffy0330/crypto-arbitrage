package telegram

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Додаємо імпорт config для типу config.Config
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	gsheets "google.golang.org/api/sheets/v4" // Потрібен для типу srv
)

// CloseUserGoal намагається оновити статус цілі в Google Sheets та видаляє ціль з пам'яті.
// Тепер приймає cfg config.Config замість spreadsheetID.
func CloseUserGoal(bot *tgbotapi.BotAPI, chatID int64, srv *gsheets.Service, cfg config.Config) { // <<< ЗМІНЕНО СИГНАТУРУ
	var msgText string

	if err != nil {
		// Якщо виникла помилка при оновленні статусу в Google Sheet
		// Використовуємо cfg.SheetNameUserGoals для більш інформативного повідомлення
		msgText = fmt.Sprintf("⚠️ Відбулася помилка під час оновлення статусу вашої цілі у Google Таблиці (аркуш '%s'): %v\n\nЦіль могла не оновитися в таблиці, але була видалена з активних у боті.", cfg.SheetNameUserGoals, err)
		log.Printf("Помилка DeleteUserGoal для ChatID %d при оновленні статусу в '%s': %v", chatID, cfg.SheetNameUserGoals, err)
	} else {
		// Якщо все успішно оновлено та видалено з пам'яті
		msgText = "✅ Вашу поточну ціль було позначено як закриту в Google Таблиці та видалено з активних у боті. Ви можете встановити нову за допомогою команди /goal."
		log.Printf("Ціль для ChatID %d успішно закрита (статус оновлено в '%s') та видалена з пам'яті.", chatID, cfg.SheetNameUserGoals)
	}

	msg := tgbotapi.NewMessage(chatID, msgText)
	if _, sendErr := bot.Send(msg); sendErr != nil {
		log.Printf("Помилка надсилання повідомлення про закриття цілі для чату %d: %v", chatID, sendErr)
	}
}
