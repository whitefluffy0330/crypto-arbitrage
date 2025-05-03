package telegram

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"
)

func StartEveningReport(bot *tgbotapi.BotAPI, srv *sheets.Service, spreadsheetID string, chatID int64) {
	go func() {
		for {
			now := time.Now()
			nextRun := time.Date(now.Year(), now.Month(), now.Day(), 22, 0, 0, 0, now.Location())
			if now.After(nextRun) {
				nextRun = nextRun.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(nextRun))
			SendEveningReport(bot, srv, spreadsheetID, chatID)
		}
	}()
}

func SendEveningReport(bot *tgbotapi.BotAPI, srv *sheets.Service, spreadsheetID string, chatID int64) {
	cfg := config.LoadEnv()
	report, err := sheets.GenerateProgressReport(srv, spreadsheetID, cfg)
	if err != nil {
		log.Printf("Помилка створення звіту: %v", err)
		return
	}

	motivation := generateMotivation()
	text := fmt.Sprintf("🌙 *Вечірній звіт*\n\n%s\n\n💬 _%s_", report, motivation)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"

	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Не вдалося надіслати звіт: %v", err)
	}
}

func generateMotivation() string {
	motivations := []string{
		"Крок за кроком до мети!",
		"Навіть маленький прогрес — це прогрес.",
		"Ти наближаєшся до своєї фінансової свободи!",
		"Результати приходять до тих, хто не зупиняється.",
		"Велике починається з малого. Ти вже в дорозі!",
		"Твоя ціль — вже ближче, ніж учора.",
		"Зусилля сьогодні — прибуток завтра!",
	}
	rand.Seed(time.Now().UnixNano())
	return motivations[rand.Intn(len(motivations))]
}
