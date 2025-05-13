package commands

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	// Додаємо імпорт пакета motivation
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

// StartWork приймає cfg та передає параметри з cfg в sheets.LogWorkStart
func StartWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, cfg config.Config) {
	chatID := msg.Chat.ID
	log.Printf("Команда /start для ChatID %d", chatID)
	startTime := time.Now() 

	err := sheets.LogWorkStart(srv, cfg.SpreadsheetID, cfg.SheetNameWorkLog, chatID, startTime)

	var text string
	if err != nil {
		log.Printf("Помилка логування початку роботи в Google Sheets для ChatID %d: %v", chatID, err)
		text = fmt.Sprintf("✅ Робочий день розпочато, але сталася помилка при записі у таблицю ('%s'): %v", cfg.SheetNameWorkLog, err)
	} else {
		text = fmt.Sprintf("✅ Робочий день розпочато о %s (за Києвом). Успішної роботи!", startTime.In(sheets.KyivLocation).Format("15:04:05"))
	}

	message := tgbotapi.NewMessage(chatID, text)
	if _, sendErr := bot.Send(message); sendErr != nil {
		log.Printf("Помилка при відправці повідомлення StartWork для ChatID %d: %v", chatID, sendErr)
	}
}

// StopWork приймає cfg та передає параметри з cfg в sheets.LogWorkStop
func StopWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, cfg config.Config) {
	chatID := msg.Chat.ID
	log.Printf("Команда /stop для ChatID %d", chatID)
	endTime := time.Now()

	duration, err := sheets.LogWorkStop(srv, cfg.SpreadsheetID, cfg.SheetNameWorkLog, chatID, endTime)

	var text string
	if err != nil {
		log.Printf("Помилка логування завершення роботи в Google Sheets для ChatID %d: %v", chatID, err)
		text = fmt.Sprintf("🛑 Робочий день завершено, але сталася помилка при записі у таблицю ('%s'): %v", cfg.SheetNameWorkLog, err)
	} else {
		durationStr := sheets.FormatDuration(duration)
		text = fmt.Sprintf("🛑 Робочий день завершено о %s (за Києвом). Тривалість: %s. Гарного відпочинку!", endTime.In(sheets.KyivLocation).Format("15:04:05"), durationStr)
	}

	message := tgbotapi.NewMessage(chatID, text)
	if _, sendErr := bot.Send(message); sendErr != nil {
		log.Printf("Помилка при відправці повідомлення StopWork для ChatID %d: %v", chatID, sendErr)
	}

	// ДОДАНО: Надсилання мотиваційної фрази після повідомлення про завершення дня
	// Переконуємося, що motivation.InitMotivationSeed() викликається в main.go
	motivationalPhrase := motivation.GetRandomMotivation()
	if motivationalPhrase != "" {
		motivationMsg := tgbotapi.NewMessage(chatID, motivationalPhrase)
		// Надсилаємо з невеликою затримкою, щоб повідомлення не "злиплися"
		// Це опціонально, можна і без затримки
		// time.Sleep(500 * time.Millisecond) 
		if _, sendErr := bot.Send(motivationMsg); sendErr != nil {
			log.Printf("Помилка при відправці мотиваційного повідомлення для ChatID %d: %v", chatID, sendErr)
		} else {
			log.Printf("Надіслано мотиваційну фразу для ChatID %d після /stop", chatID)
		}
	}
}

// DayOff приймає cfg та передає параметри з cfg в sheets.LogDayOff
func DayOff(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, cfg config.Config) {
	chatID := msg.Chat.ID
	log.Printf("Команда /dayoff для ChatID %d", chatID)
	dateToLog := time.Now() 

	err := sheets.LogDayOff(srv, cfg.SpreadsheetID, cfg.SheetNameWorkLog, chatID, dateToLog)

	var text string
	if err != nil {
		log.Printf("Помилка логування вихідного дня в Google Sheets для ChatID %d: %v", chatID, err)
		text = fmt.Sprintf("📅 Сьогодні вихідний. Сталася помилка при записі у таблицю ('%s'): %v", cfg.SheetNameWorkLog, err)
	} else {
		text = fmt.Sprintf("📅 Статус 'Вихідний' на %s (за Києвом) встановлено в таблиці '%s'.", dateToLog.In(sheets.KyivLocation).Format("02.01.2006"), cfg.SheetNameWorkLog)
	}

	message := tgbotapi.NewMessage(chatID, text)
	if _, sendErr := bot.Send(message); sendErr != nil {
		log.Printf("Помилка при відправці повідомлення DayOff для ChatID %d: %v", chatID, sendErr)
	}
}
