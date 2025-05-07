package commands

import (
	"fmt" // Додано для форматування повідомлень
	"log"
	"time" // Додано для роботи з часом

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Імпортуємо пакет sheets для виклику функцій роботи з таблицею
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4"
)

// StartWork тепер логує початок роботи в Google Sheets
func StartWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, spreadsheetID string) {
	log.Printf("Команда /start для ChatID %d", msg.Chat.ID)
	now := time.Now().UTC() // Використовуємо UTC для універсальності
	
	// TODO: Реалізувати функцію sheets.LogWorkStart в пакеті sheets
	err := sheets.LogWorkStart(srv, spreadsheetID, msg.Chat.ID, now) // Передаємо час початку

	var text string
	if err != nil {
		log.Printf("Помилка логування початку роботи в Google Sheets для ChatID %d: %v", msg.Chat.ID, err)
		text = fmt.Sprintf("✅ Робочий день розпочато, але сталася помилка при записі у таблицю: %v", err)
	} else {
		text = fmt.Sprintf("✅ Робочий день розпочато о %s (UTC). Успішної роботи!", now.Format("15:04:05"))
	}
	
	message := tgbotapi.NewMessage(msg.Chat.ID, text)
	if _, sendErr := bot.Send(message); sendErr != nil {
		log.Printf("Помилка при відправці повідомлення StartWork для ChatID %d: %v", msg.Chat.ID, sendErr)
	}
}

// StopWork тепер логує завершення роботи та тривалість в Google Sheets
func StopWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, spreadsheetID string) {
	log.Printf("Команда /stop для ChatID %d", msg.Chat.ID)
	now := time.Now().UTC()

	// TODO: Реалізувати функцію sheets.LogWorkStop в пакеті sheets
	duration, err := sheets.LogWorkStop(srv, spreadsheetID, msg.Chat.ID, now) // Передаємо час завершення

	var text string
	if err != nil {
		log.Printf("Помилка логування завершення роботи в Google Sheets для ChatID %d: %v", msg.Chat.ID, err)
		text = fmt.Sprintf("🛑 Робочий день завершено, але сталася помилка при записі у таблицю: %v", err)
	} else {
		// Форматуємо тривалість для читабельності
		durationStr := duration.Truncate(time.Second).String() // Наприклад, "1h2m3s"
		text = fmt.Sprintf("🛑 Робочий день завершено о %s (UTC). Тривалість: %s. Гарного відпочинку!", now.Format("15:04:05"), durationStr)
	}

	message := tgbotapi.NewMessage(msg.Chat.ID, text)
	if _, sendErr := bot.Send(message); sendErr != nil {
		log.Printf("Помилка при відправці повідомлення StopWork для ChatID %d: %v", msg.Chat.ID, sendErr)
	}
}

// DayOff тепер логує вихідний день в Google Sheets
func DayOff(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, spreadsheetID string) {
	log.Printf("Команда /dayoff для ChatID %d", msg.Chat.ID)
	now := time.Now().UTC()

	// TODO: Реалізувати функцію sheets.LogDayOff в пакеті sheets
	err := sheets.LogDayOff(srv, spreadsheetID, msg.Chat.ID, now.Format("2006-01-02")) // Передаємо дату

	var text string
	if err != nil {
		log.Printf("Помилка логування вихідного дня в Google Sheets для ChatID %d: %v", msg.Chat.ID, err)
		text = fmt.Sprintf("📅 Сьогодні вихідний. Сталася помилка при записі у таблицю: %v", err)
	} else {
		text = "📅 Статус 'Вихідний' на сьогодні встановлено в таблиці."
	}

	message := tgbotapi.NewMessage(msg.Chat.ID, text)
	if _, sendErr := bot.Send(message); sendErr != nil {
		log.Printf("Помилка при відправці повідомлення DayOff для ChatID %d: %v", msg.Chat.ID, sendErr)
	}
}
