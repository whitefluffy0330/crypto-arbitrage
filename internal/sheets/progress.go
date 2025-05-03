package sheets

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// NewService створює новий клієнт Google Sheets
func NewService(credentialsJSON []byte) (*sheets.Service, error) {
	ctx := context.Background()
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(credentialsJSON))
	if err != nil {
		return nil, fmt.Errorf("не вдалося створити клієнт Sheets: %w", err)
	}
	return srv, nil
}

func GenerateProgressReport(srv *sheets.Service, spreadsheetID string) string {
	readRange := "ЩоденнийЗвіт!A2:E2"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Не вдалося отримати дані з Google Sheets: %v", err)
		return "Помилка отримання звіту."
	}

	if len(resp.Values) < 1 || len(resp.Values[0]) < 5 {
		return "Недостатньо даних для формування звіту."
	}

	row := resp.Values[0]
	date := row[0]
	income := row[1]
	goal := row[2]
	daysLeft := row[3]
	requiredDaily := row[4]

	return fmt.Sprintf(
		"📅 Дата: %v\n💰 Заробіток: %v$\n🎯 Мета: %v$\n🕒 Днів до кінця місяця: %v\n📈 Потрібно заробляти щодня: %v$\n\n🔥 %s",
		date, income, goal, daysLeft, requiredDaily, getMotivation(),
	)
}

func getMotivation() string {
	now := time.Now()
	hour := now.Hour()

	switch {
	case hour < 12:
		return "Почни цей день потужно — результат не забариться!"
	case hour < 18:
		return "Тримай темп, ти вже ближче до мети!"
	default:
		return "Завершуй день із гордістю за зроблене!"
	}
}
