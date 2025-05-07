package sheets

import (
	"context"
	"fmt"
	"log"
	"strconv" // Для конвертації рядків у числа
	"time"
	// "strings" // Може знадобитися для додаткової обробки рядків

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets.readonly"

// SheetRowData структура для зберігання даних, прочитаних з одного рядка таблиці
type SheetRowData struct {
	Date            string  // Дата з таблиці
	Income          float64 // Фактичний дохід
	SheetGoal       float64 // План/Мета з таблиці
	SheetDaysLeft   int     // Залишилося днів у періоді з таблиці
	SheetReqDaily   float64 // Необхідно щодня з таблиці
}

// NewService (ймовірно, не використовується)
func NewService(credentialsJSON []byte) (*sheets.Service, error) {
	ctx := context.Background()
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(credentialsJSON))
	if err != nil {
		return nil, fmt.Errorf("не вдалося створити клієнт Sheets: %w", err)
	}
	return srv, nil
}

// GenerateProgressReport тепер повертає структуровані дані SheetRowData та помилку
func GenerateProgressReport(srv *sheets.Service, spreadsheetID string) (SheetRowData, error) {
	readRange := "Звіт!A2:E2" // Або інший аркуш/діапазон, якщо потрібно
	var data SheetRowData

	log.Printf("Спроба читання даних з Google Sheets: SpreadsheetID=%s, Range=%s", spreadsheetID, readRange)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Не вдалося отримати дані з Google Sheets: %v", err)
		return data, fmt.Errorf("помилка отримання даних з Google Sheets: %w", err)
	}

	if len(resp.Values) < 1 || len(resp.Values[0]) < 5 {
		errMsg := fmt.Sprintf("недостатньо даних у діапазоні %s: отримано %d рядків (або недостатньо колонок), очікувався 1 рядок з 5 колонками", readRange, len(resp.Values))
		log.Println(errMsg)
		return data, fmt.Errorf(errMsg)
	}

	row := resp.Values[0] // Беремо перший рядок даних (має бути рядок A2:E2)

	// Парсинг даних з рядка
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) }
	
	if len(row) > 1 { 
		incomeStr := fmt.Sprintf("%v", row[1])
		data.Income, err = strconv.ParseFloat(incomeStr, 64)
		if err != nil {
			log.Printf("Помилка парсингу доходу '%s': %v", incomeStr, err)
			return data, fmt.Errorf("некоректний формат доходу в таблиці: %s", incomeStr)
		}
	}
	if len(row) > 2 {
		sheetGoalStr := fmt.Sprintf("%v", row[2])
		data.SheetGoal, err = strconv.ParseFloat(sheetGoalStr, 64)
		if err != nil {
			log.Printf("Помилка парсингу мети з таблиці '%s': %v", sheetGoalStr, err)
			return data, fmt.Errorf("некоректний формат мети в таблиці: %s", sheetGoalStr)
		}
	}
	if len(row) > 3 {
		sheetDaysLeftStr := fmt.Sprintf("%v", row[3])
		data.SheetDaysLeft, err = strconv.Atoi(sheetDaysLeftStr)
		if err != nil {
			log.Printf("Помилка парсингу 'днів залишилося' з таблиці '%s': %v", sheetDaysLeftStr, err)
			return data, fmt.Errorf("некоректний формат 'днів залишилося' в таблиці: %s", sheetDaysLeftStr)
		}
	}
	if len(row) > 4 {
		sheetReqDailyStr := fmt.Sprintf("%v", row[4])
		data.SheetReqDaily, err = strconv.ParseFloat(sheetReqDailyStr, 64)
		if err != nil {
			log.Printf("Помилка парсингу 'потрібно щодня' з таблиці '%s': %v", sheetReqDailyStr, err)
			return data, fmt.Errorf("некоректний формат 'потрібно щодня' в таблиці: %s", sheetReqDailyStr)
		}
	}

	log.Printf("Дані з таблиці успішно розпарсені: %+v", data)
	return data, nil // Повертаємо структуровані дані та відсутність помилки
}

// getMotivation (залишається без змін, але зараз не використовується цією функцією)
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
