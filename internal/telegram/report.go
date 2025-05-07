package telegram

import (
	"fmt"  // Додано для форматування звіту
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Розкоментовуємо імпорт вашого пакету sheets, оскільки будемо його викликати
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4" // Використовуємо gsheets для типу srv *gsheets.Service
)

// ReportProgress надсилає звіт про прогрес користувачеві.
func ReportProgress(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, spreadsheetID string) {
	chatID := msg.Chat.ID
	var reportText string

	currentGoal, goalExists := GetUserGoal(chatID) // Отримуємо поточну ціль користувача

	if !goalExists {
		reportText = "🎯 Спочатку вам потрібно встановити фінансову ціль за допомогою команди /goal або кнопки \"🎯 Моя ціль\"."
		log.Printf("Запит звіту для ChatID %d, але ціль не встановлена.", chatID)
	} else {
		log.Printf("Генерація звіту для ChatID %d з ціллю: %+v", chatID, currentGoal)
		// Отримуємо дані з Google Sheets (поки що sheets.GenerateProgressReport є заглушкою)
		// У майбутньому sheets.GenerateProgressReport має повертати структуровані дані або детальний звіт.
		sheetDataReport := sheets.GenerateProgressReport(srv, spreadsheetID) // Викликаємо функцію з вашого пакета sheets

		// Формуємо звіт, поєднуючи дані цілі та дані з таблиці
		reportText = fmt.Sprintf(
			"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
				"🎯 **Поточна Ціль:**\n"+
				"   Сума: %.2f %s\n"+
				"   Термін: %d днів\n"+
				"   Встановлено: %s\n\n"+
				"📈 **Дані з Google Sheets:**\n"+
				"%s\n\n"+ // Тут буде текст, повернутий GenerateProgressReport
				"Продовжуйте в тому ж дусі!",
			currentGoal.Amount, currentGoal.Currency, currentGoal.Days, currentGoal.SetDate.Format("02.01.2006"),
			sheetDataReport,
		)
		// Для Markdown V2 потрібно екранувати деякі символи, але для простого тексту або HTML це не обов'язково.
		// Якщо будете використовувати складний Markdown, розгляньте tgbotapi.ModeMarkdownV2
	}

	response := tgbotapi.NewMessage(chatID, reportText)
	response.ParseMode = tgbotapi.ModeMarkdown // Встановлюємо Markdown для форматування
	if _, err := bot.Send(response); err != nil {
		log.Printf("Помилка надсилання звіту про прогрес для чату %d: %v", chatID, err)
	}
}
