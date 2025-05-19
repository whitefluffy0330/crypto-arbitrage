package commands

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	// gsheets "google.golang.org/api/sheets/v4" // Не потрібен, якщо srv це *sheets.Service
)

func StartWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *sheets.Service, cfg *config.Config) { // Змінено тип srv та cfg
	chatID := msg.Chat.ID
	log.Printf("Команда /start для ChatID %d", chatID)
	startTime := time.Now() 
	// KyivLocation має бути доступний з пакета telegram, якщо він там експортований,
	// або переданий через параметр, або використаний з sheets.KyivLocation, якщо sheets імпортовано.
	// Припускаючи, що KyivLocation доступний глобально з telegram.go
	err := sheets.LogWorkStart(srv, cfg.SpreadsheetID, cfg.SheetNameWorkLog, chatID, startTime)
	var text string
	if err != nil {
		text = fmt.Sprintf("✅ Робочий день розпочато, але помилка запису: %v", err)
	} else {
		text = fmt.Sprintf("✅ Робочий день розпочато о %s. Успіхів!", startTime.In(telegram.KyivLocation).Format("15:04:05")) // Використовуємо telegram.KyivLocation
	}
	message := tgbotapi.NewMessage(chatID, text)
	if _, sendErr := bot.Send(message); sendErr != nil {
		log.Printf("Помилка StartWork send: %v", sendErr)
	}
}

func StopWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *sheets.Service, cfg *config.Config) { // Змінено тип srv та cfg
	chatID := msg.Chat.ID
	log.Printf("Команда /stop для ChatID %d", chatID)
	endTime := time.Now()
	duration, err := sheets.LogWorkStop(srv, cfg.SpreadsheetID, cfg.SheetNameWorkLog, chatID, endTime)
	var text string
	if err != nil {
		text = fmt.Sprintf("🛑 День завершено, помилка запису: %v", err)
	} else {
		durationStr := sheets.FormatDuration(duration)
		text = fmt.Sprintf("🛑 День завершено о %s. Тривалість: %s.", endTime.In(telegram.KyivLocation).Format("15:04:05"), durationStr) // Використовуємо telegram.KyivLocation
	}
	message := tgbotapi.NewMessage(chatID, text)
	bot.Send(message) // Ігноруємо помилку відправки для простоти

	motivationalPhrase := motivation.GetRandomMotivation()
	if motivationalPhrase != "" {
		motivationMsg := tgbotapi.NewMessage(chatID, motivationalPhrase)
		if _, sendErr := bot.Send(motivationMsg); sendErr != nil {
			log.Printf("Помилка мотивації: %v", sendErr)
		}
	}
}

func DayOff(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *sheets.Service, cfg *config.Config) { // Змінено тип srv та cfg
	chatID := msg.Chat.ID
	log.Printf("Команда /dayoff для ChatID %d", chatID)
	dateToLog := time.Now() 
	err := sheets.LogDayOff(srv, cfg.SpreadsheetID, cfg.SheetNameWorkLog, chatID, dateToLog)
	var text string
	if err != nil {
		text = fmt.Sprintf("📅 Вихідний. Помилка запису: %v", err)
	} else {
		text = fmt.Sprintf("📅 Статус 'Вихідний' на %s встановлено.", dateToLog.In(telegram.KyivLocation).Format("02.01.2006")) // Використовуємо telegram.KyivLocation
	}
	message := tgbotapi.NewMessage(chatID, text)
	bot.Send(message)
}
