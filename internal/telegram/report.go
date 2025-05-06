package telegram

import (
	"log" // Додано для логування помилок

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Тимчасово коментуємо імпорт вашого пакету sheets,
	// оскільки функція sheets.GenerateProgressReport ще не викликається.
	// Коли ви будете готові реалізувати отримання даних для звіту,
	// потрібно буде розкоментувати цей імпорт.
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4" // Використовуємо gsheets для типу srv *gsheets.Service
)

// ReportProgress надсилає звіт про прогрес користувачеві.
func ReportProgress(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, spreadsheetID string) {
	var reportText string

	// TODO: Реалізувати повну логіку звіту.
	// Коли функція sheets.GenerateProgressReport з вашого пакету "internal/sheets"
	// буде готова і повертатиме рядок, ви можете розкоментувати наступний рядок
	// ТА розкоментувати імпорт "github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" вище:
	// reportText = sheets.GenerateProgressReport(srv, spreadsheetID)

	// Поки що використовуємо оновлену заглушку:
	if reportText == "" { // Якщо реальний звіт не був згенерований
		reportText = "📊 Звіт про прогрес:\n\n(Наразі функція отримання даних з Google Sheets для цього звіту в розробці. Скоро тут будуть ваші актуальні цифри!)"
	}

	response := tgbotapi.NewMessage(msg.Chat.ID, reportText)
	if _, err := bot.Send(response); err != nil {
		log.Printf("Помилка надсилання звіту про прогрес для чату %d: %v", msg.Chat.ID, err)
	}
}
