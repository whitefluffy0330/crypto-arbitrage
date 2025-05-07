package telegram

import (
	"fmt" // Додано для форматування повідомлень
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Додаємо імпорт gsheets для типу srv у сигнатурі CloseUserGoal
	gsheets "google.golang.org/api/sheets/v4"
)

// CloseUserGoal намагається оновити статус цілі в Google Sheets на "Закрита"
// та видаляє ціль користувача з пам'яті бота.
// Тепер приймає srv та spreadsheetID.
// TODO: Потрібно визначити, яка команда/кнопка буде викликати цю функцію з handler.go
// (ми вже передбачили case "/closegoal", "❌ Закрити ціль" у handler.go).
func CloseUserGoal(bot *tgbotapi.BotAPI, chatID int64, srv *gsheets.Service, spreadsheetID string) {
	var msgText string

	// Викликаємо оновлену DeleteUserGoal, передаючи srv та spreadsheetID, та обробляємо помилку
	err := DeleteUserGoal(chatID, srv, spreadsheetID)

	if err != nil {
		// Якщо виникла помилка при оновленні статусу в Google Sheet
		msgText = fmt.Sprintf("⚠️ Відбулася помилка під час оновлення статусу вашої цілі у Google Таблиці: %v\n\nЦіль могла не оновитися в таблиці, але була видалена з активних у боті. Спробуйте пізніше або перевірте таблицю.", err)
		log.Printf("Помилка DeleteUserGoal для ChatID %d при оновленні статусу в Google Sheet: %v", chatID, err)
	} else {
		// Якщо все успішно оновлено (включно з Google Sheet) та видалено з пам'яті
		msgText = "✅ Вашу поточну ціль було позначено як закриту в Google Таблиці та видалено з активних у боті. Ви можете встановити нову за допомогою команди /goal."
		log.Printf("Ціль для ChatID %d успішно закрита (статус оновлено в Google Sheets) та видалена з пам'яті.", chatID)
	}

	msg := tgbotapi.NewMessage(chatID, msgText)
	if _, sendErr := bot.Send(msg); sendErr != nil { // Перейменовано змінну помилки, щоб уникнути затінення
		log.Printf("Помилка надсилання повідомлення про закриття цілі для чату %d: %v", chatID, sendErr)
	}
}
