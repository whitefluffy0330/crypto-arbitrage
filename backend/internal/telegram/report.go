package telegram

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage-bot/internal/sheets"
)

func StartEveningReport(bot *tgbotapi.BotAPI, srv *sheets.Service, spreadsheetID string, chatID int64) {
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, now.Location()) // 20:00
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(next.Sub(now))
			SendEveningReport(bot, srv, spreadsheetID, chatID)
		}
	}()
}

func SendEveningReport(bot *tgbotapi.BotAPI, srv *sheets.Service, spreadsheetID string, chatID int64) {
	progressText, err := sheets.GenerateProgressReport(srv, spreadsheetID)
	if err != nil {
		log.Printf("Помилка створення звіту: %v", err)
		return
	}

	motivations := []string{
		"🏆 Велика мета складається з маленьких перемог. І сьогодні ти її наблизив!",
		"🚀 Ще один день продуктивності — ще один крок до твоєї цілі!",
		"🔥 Пам'ятай навіщо почав. Ти молодець!",
		"💪 Кожен день — це фундамент твого майбутнього.",
		"🎯 Навіть 1% прогресу щодня — це 37x за рік!",
	}

	text := fmt.Sprintf("📋 Звіт за день:\n%s\n\n%s", progressText, motivations[rand.Intn(len(motivations))])

	msg := tgbotapi.NewMessage(chatID, text)
	bot.Send(msg)
}
