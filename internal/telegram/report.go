package telegram

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
)

var eveningReportSent bool

func StartEveningReport(bot *tgbotapi.BotAPI, srv *sheets.Service, spreadsheetID string, chatID int64, enableReport bool) {
	go func() {
		for {
			now := time.Now()
			location := now.Location()
			evening := time.Date(now.Year(), now.Month(), now.Day(), 22, 0, 0, 0, location)

			if now.After(evening) {
				evening = evening.Add(24 * time.Hour)
				eveningReportSent = false
			}

			duration := evening.Sub(now)
			timer := time.NewTimer(duration)
			<-timer.C

			if !eveningReportSent {
				report := sheets.GenerateProgressReport(srv, spreadsheetID)
				SendEveningReport(bot, chatID, report, enableReport)
				eveningReportSent = true
			}
		}
	}()
}

func SendEveningReport(bot *tgbotapi.BotAPI, chatID int64, report string, enableReport bool) {
	if !enableReport {
		log.Println("Вечірній звіт вимкнено через налаштування")
		return
	}

	message := tgbotapi.NewMessage(chatID, fmt.Sprintf("\u2728 *Щоденний звіт*\n\n%s", report))
	message.ParseMode = "Markdown"

	_, err := bot.Send(message)
	if err != nil {
		log.Printf("Не вдалося надіслати звіт: %v", err)
	}
}
