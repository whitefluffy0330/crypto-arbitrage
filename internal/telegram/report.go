package telegram

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
)

func SendProgressReport(bot *tgbotapi.BotAPI, chatID int64, srv *sheets.Service, spreadsheetID string) {
	report, err := sheets.GenerateProgressReport(srv, spreadsheetID)
	if err != nil {
		log.Printf("Помилка при генерації звіту: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "Сталася помилка при формуванні звіту 📉"))
		return
	}

	motivation := getMotivationalQuote()
	messageText := fmt.Sprintf("%s\n\n📈 %s", motivation, report)

	msg := tgbotapi.NewMessage(chatID, messageText)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Не вдалося надіслати звіт: %v", err)
	}
}

func getMotivationalQuote() string {
	quotes := []string{
		"🚀 Кожен крок — це наближення до мети!",
		"🔥 Не зупиняйся зараз — ти вже близько!",
		"🏆 Твоя дисципліна — твоя суперсила!",
		"📊 Маленький дохід — теж дохід!",
		"🎯 Вчора — досвід, сьогодні — прогрес.",
	}
	rand.Seed(time.Now().UnixNano())
	return quotes[rand.Intn(len(quotes))]
}
