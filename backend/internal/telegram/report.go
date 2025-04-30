package telegram

import (
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"google.golang.org/api/sheets/v4"
)

func StartEveningReport(bot *tgbotapi.BotAPI, srv *sheets.Service, spreadsheetID string, chatID int64) {
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 21, 0, 0, 0, now.Location())
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(next))

			text := generateReport(srv, spreadsheetID)
			msg := tgbotapi.NewMessage(chatID, text)
			bot.Send(msg)
		}
	}()
}

func generateReport(srv *sheets.Service, spreadsheetID string) string {
	readRange := "Звіт!A:C"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil || len(resp.Values) < 2 {
		return "📋 Немає даних для звіту."
	}

	last := resp.Values[len(resp.Values)-1]
	text := "📋 Звіт за день:\n"
	if len(last) >= 3 {
		status := strings.TrimSpace(fmt.Sprintf("%s", last[1]))
		timeSpent := strings.TrimSpace(fmt.Sprintf("%s", last[2]))
		text += fmt.Sprintf("✅ Статус: %s\n🕒 Час роботи: %s\n", status, timeSpent)
	} else {
		text += "Немає повного запису про сьогодні."
	}

	daysLeft := daysLeftInMonth()
	text += fmt.Sprintf("\n📆 До кінця місяця: %d днів\n", daysLeft)
	text += motivationalEnding()
	return text
}

func daysLeftInMonth() int {
	now := time.Now()
	year, month := now.Year(), now.Month()
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, now.Location())
	return lastDay.Day() - now.Day()
}

func motivationalEnding() string {
	messages := []string{
		"🏆 Велика мета складається з маленьких перемог!",
		"🔥 Завтра — ще одна можливість для прориву!",
		"💪 Ти просуваєшся до мети, не зупиняйся!",
		"🚀 Ще один день у правильному напрямку!",
	}
	return "\n" + messages[time.Now().UnixNano()%int64(len(messages))]
}
