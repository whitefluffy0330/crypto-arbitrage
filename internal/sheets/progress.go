package sheets

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets.readonly"

// NewService (ймовірно, не використовується, як ми обговорювали)
func NewService(credentialsJSON []byte) (*sheets.Service, error) {
	ctx := context.Background()
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(credentialsJSON))
	if err != nil {
		return nil, fmt.Errorf("не вдалося створити клієнт Sheets: %w", err)
	}
	return srv, nil
}

func GenerateProgressReport(srv *sheets.Service, spreadsheetID string) string {
	// ЗМІНЕНО НАЗВУ АРКУША з "ЩоденнийЗвіт" на "Звіт"
	readRange := "Звіт!A2:E2" // <--- ОСНОВНА ЗМІНА ТУТ!

	// Якщо ваш звіт насправді на іншому аркуші (наприклад, "Мій_Прогресу"),
	// вкажіть тут його назву. Також переконайтеся, що діапазон A2:E2
	// на цьому аркуші містить 5 значень, які очікує код (дата, дохід, мета, днів залишилося, потрібно щодня).

	log.Printf("Спроба читання даних з Google Sheets: SpreadsheetID=%s, Range=%s", spreadsheetID, readRange)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Не вдалося отримати дані з Google Sheets: %v", err)
		return "Помилка отримання даних зі звіту." // Змінено текст помилки
	}

	if len(resp.Values) < 1 || len(resp.Values[0]) < 5 {
		log.Printf("Недостатньо даних для звіту в діапазоні %s: отримано %d рядків, очікувався хоча б 1 рядок з 5 колонками", readRange, len(resp.Values))
		return "Недостатньо даних у таблиці для формування звіту." // Змінено текст помилки
	}

	row := resp.Values[0]
	var date, income, goal, daysLeft, requiredDaily interface{}

	if len(row) > 0 { date = row[0] }
	if len(row) > 1 { income = row[1] }
	if len(row) > 2 { goal = row[2] }
	if len(row) > 3 { daysLeft = row[3] }
	if len(row) > 4 { requiredDaily = row[4] }

	log.Printf("Дані з таблиці отримано: Дата=%v, Дохід=%v, Мета=%v, ДнівЗалишилось=%v, ПотрібноЩодня=%v",
		date, income, goal, daysLeft, requiredDaily)

	return fmt.Sprintf(
		"📅 Дата з таблиці: %v\n💰 Ваш дохід з таблиці: %v\n🎯 Ваша мета з таблиці: %v\n🕒 Днів до кінця (з таблиці): %v\n📈 Потрібно заробляти щодня (з таблиці): %v\n\n🔥 %s",
		date, income, goal, daysLeft, requiredDaily, getMotivation(), // getMotivation() визначена нижче
	)
}

func getMotivation() string {
	hour := time.Now().Hour()
	switch {
	case hour < 12:
		return "Почни цей день потужно — результат не забариться!"
	case hour < 18:
		return "Тримай темп, ти вже ближче до мети!"
	default:
		return "Завершуй день із гордістю за зроблене!"
	}
}
