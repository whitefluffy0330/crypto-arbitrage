package sheets

import (
	"fmt"
	"log"
	"time"

	"google.golang.org/api/sheets/v4"
)

func GenerateProgressReport(srv *sheets.Service, spreadsheetID string) string {
	readRange := "ЩоденнийЗвіт!A1:E2"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Не вдалося отримати дані з Google Sheets: %v", err)
		return "Помилка отримання звіту."
	}

	if len(resp.Values) < 2 {
		return "Недостатньо даних для формування звіту."
	}

	values := resp.Values[1]
	if len(values) < 5 {
		return "Недостатньо стовпців у даних звіту."
	}

	date := values[0]
	income := values[1]
	goal := values[2]
	daysLeft := values[3]
	requiredDaily := values[4]

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
