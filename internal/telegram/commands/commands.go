package commands

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4"
)

// getCurrentTimeInKyiv (копія з sheets.go, або краще винести в окремий shared утилітний пакет, якщо буде багато таких)
// Або просто передавати time.Now() і нехай пакет sheets сам розбирається з часовою зоною
var kyivLocationCommands *time.Location 

func init() {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Printf("Критична помилка в commands: не вдалося завантажити часову зону Europe/Kyiv: %v. Буде використано UTC.", err)
		kyivLocationCommands = time.UTC
	} else {
		kyivLocationCommands = loc
	}
}

func getCurrentTimeInKyivCommands() time.Time {
	return time.Now().In(kyivLocationCommands)
}


func StartWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, spreadsheetID string) {
	chatID := msg.Chat.ID
	log.Printf("Команда /start для ChatID %d", chatID)
	// Передаємо поточний час. Пакет sheets подбає про часову зону.
	startTime := time.Now() 
	
	err := sheets.LogWorkStart(srv, spreadsheetID, chatID, startTime)

	var text string
	if err != nil {
		log.Printf("Помилка логування початку роботи в Google Sheets для ChatID %d: %v", chatID, err)
		text = fmt.Sprintf("✅ Робочий день розпочато, але сталася помилка при записі у таблицю: %v", err)
	} else {
		// Для відображення користувачу, конвертуємо час у Київський
		text = fmt.Sprintf("✅ Робочий день розпочато о %s (за Києвом). Успішної роботи!", startTime.In(kyivLocationCommands).Format("15:04:05"))
	}
	
	message := tgbotapi.NewMessage(chatID, text)
	if _, sendErr := bot.Send(message); sendErr != nil {
		log.Printf("Помилка при відправці повідомлення StartWork для ChatID %d: %v", chatID, sendErr)
	}
}

func StopWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, spreadsheetID string) {
	chatID := msg.Chat.ID
	log.Printf("Команда /stop для ChatID %d", chatID)
	endTime := time.Now()

	duration, err := sheets.LogWorkStop(srv, spreadsheetID, chatID, endTime)

	var text string
	if err != nil {
		log.Printf("Помилка логування завершення роботи в Google Sheets для ChatID %d: %v", chatID, err)
		text = fmt.Sprintf("🛑 Робочий день завершено, але сталася помилка при записі у таблицю: %v", err)
	} else {
		text = fmt.Sprintf("🛑 Робочий день завершено о %s (за Києвом). Тривалість: %s. Гарного відпочинку!", endTime.In(kyivLocationCommands).Format("15:04:05"), sheets.FormatDuration(duration)) // Використовуємо sheets.FormatDuration
	}

	message := tgbotapi.NewMessage(chatID, text)
	if _, sendErr := bot.Send(message); sendErr != nil {
		log.Printf("Помилка при відправці повідомлення StopWork для ChatID %d: %v", chatID, sendErr)
	}
}

func DayOff(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, spreadsheetID string) {
	chatID := msg.Chat.ID
	log.Printf("Команда /dayoff для ChatID %d", chatID)
	dateToLog := time.Now() // Функція LogDayOff в sheets сама розбереться з датою та часовою зоною

	err := sheets.LogDayOff(srv, spreadsheetID, chatID, dateToLog)

	var text string
	if err != nil {
		log.Printf("Помилка логування вихідного дня в Google Sheets для ChatID %d: %v", chatID, err)
		text = fmt.Sprintf("📅 Сьогодні вихідний. Сталася помилка при записі у таблицю: %v", err)
	} else {
		text = fmt.Sprintf("📅 Статус 'Вихідний' на %s (за Києвом) встановлено в таблиці.", dateToLog.In(kyivLocationCommands).Format("02.01.2006"))
	}

	message := tgbotapi.NewMessage(chatID, text)
	if _, sendErr := bot.Send(message); sendErr != nil {
		log.Printf("Помилка при відправці повідомлення DayOff для ChatID %d: %v", chatID, sendErr)
	}
}
