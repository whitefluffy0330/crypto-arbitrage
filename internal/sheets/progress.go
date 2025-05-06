package sheets

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Додано константу SpreadsheetsScope
const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets.readonly"
// Якщо вам потрібен також доступ на запис до таблиць, використовуйте:
// const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets"

// NewService створює новий клієнт Google Sheets.
// Як ми обговорювали, ця функція, ймовірно, не використовується у вашому main.go,
// оскільки main.go використовує google.FindDefaultCredentials для автентифікації.
// Можливо, її варто буде переглянути або видалити під час рефакторингу.
func NewService(credentialsJSON []byte) (*sheets.Service, error) {
	ctx := context.Background()
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(credentialsJSON))
	if err != nil {
		return nil, fmt.Errorf("не вдалося створити клієнт Sheets: %w", err)
	}
	return srv, nil
}

// GenerateProgressReport генерує текстовий звіт про прогрес.
func GenerateProgressReport(srv *sheets.Service, spreadsheetID string) string {
	// Назва аркуша та діапазон жорстко закодовані.
	// Розгляньте можливість зробити їх конфігурованими або передавати як параметри.
	readRange := "ЩоденнийЗвіт!A2:E2"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Не вдалося отримати дані з Google Sheets: %v", err)
		return "Помилка отримання звіту."
	}

	if len(resp.Values) < 1 || len(resp.Values[0]) < 5 {
		// log.Printf("Недостатньо даних для звіту: отримано %d рядків, очікувався хоча б 1 рядок з 5 колонками", len(resp.Values))
		return "Недостатньо даних для формування звіту."
	}

	row := resp.Values[0]
	// Додамо перевірки на кількість елементів у рядку, щоб уникнути паніки
	var date, income, goal, daysLeft, requiredDaily interface{} // Використовуємо interface{} для безпечного доступу

	if len(row) > 0 { date = row[0] }
	if len(row) > 1 { income = row[1] }
	if len(row) > 2 { goal = row[2] }
	if len(row) > 3 { daysLeft = row[3] }
	if len(row) > 4 { requiredDaily = row[4] }


	// Функція getMotivation() визначена нижче.
	// Можливо, варто перенести логіку мотиваційних повідомлень
	// ближче до формування відповіді в пакеті telegram.
	return fmt.Sprintf(
		"📅 Дата: %v\n💰 Заробіток: %v$\n🎯 Мета: %v$\n🕒 Днів до кінця місяця: %v\n📈 Потрібно заробляти щодня: %v$\n\n🔥 %s",
		date, income, goal, daysLeft, requiredDaily, getMotivation(),
	)
}

// getMotivation повертає мотиваційну фразу залежно від часу доби.
func getMotivation() string {
	hour := time.Now().Hour() // Використовує поточний час сервера

	switch {
	case hour < 12:
		return "Почни цей день потужно — результат не забариться!"
	case hour < 18:
		return "Тримай темп, ти вже ближче до мети!"
	default:
		return "Завершуй день із гордістю за зроблене!"
	}
}
