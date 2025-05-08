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

const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets"

// ---- Структури ----
type SheetRowData struct {
	Date          string
	Income        float64
	SheetGoal     float64
	SheetDaysLeft int
	SheetReqDaily float64
}
type FinancialGoalData struct {
	Amount       float64
	Currency     string
	Days         int
	OriginalText string
	SetDate      time.Time
}

// ---- Часова зона ----
var KyivLocation *time.Location
func init() { /* ... код без змін ... */ 
	loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Критична помилка: не вдалося завантажити часову зону Europe/Kyiv: %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv успішно завантажено (пакет sheets).") }
}
func GetCurrentTimeInKyiv() time.Time { return time.Now().In(KyivLocation) }

// ---- Допоміжні функції ----
func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) (int, []interface{}) { /* ... код без змін ... */ 
	if len(sheetData) == 0 { return -1, nil }; startRowIndex := 0; if len(sheetData) > 0 && len(sheetData[0]) > 0 && (fmt.Sprintf("%v", sheetData[0][0]) == "Дата" || fmt.Sprintf("%v", sheetData[0][0]) == "Date" || fmt.Sprintf("%v", sheetData[0][0]) == "ChatID" || fmt.Sprintf("%v", sheetData[0][0]) == "ID чату користувача") { startRowIndex = 1 }; for i := startRowIndex; i < len(sheetData); i++ { row := sheetData[i]; if len(row) > 0 { if fmt.Sprintf("%v", row[0]) == dateToFind { return i, row } } }; return -1, nil
}
func FormatDuration(d time.Duration) string { /* ... код без змін ... */ 
	d = d.Round(time.Minute); h := d / time.Hour; d -= h * time.Hour; m := d / time.Minute; return fmt.Sprintf("%dh %dm", h, m)
}

// --- Функції роботи з Google Sheets API ---
func NewService(credentialsJSON []byte) (*sheets.Service, error) { /*...*/ return nil, nil }

// GenerateProgressReport тепер читає діапазон A2:E та перевіряє перший рядок
func GenerateProgressReport(srv *sheets.Service, spreadsheetID string, reportSheetNameAndRange string) (SheetRowData, error) {
	// reportSheetNameAndRange тепер повний рядок, напр. "Звіт!A2:E2"
	// Розділимо його на назву аркуша та діапазон для нової логіки читання
	sheetName := "Звіт" // Значення за замовчуванням
	sheetRange := "A2:E" // Читаємо від A2 до E до кінця
	
	// Спробуємо розділити отриманий рядок, якщо він заданий через конфіг
	parts := strings.Split(reportSheetNameAndRange, "!")
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		sheetName = parts[0]
		// Змінюємо діапазон читання на "A2:E" для цього аркуша
		sheetRange = fmt.Sprintf("%s!A2:E", sheetName) // Формуємо новий діапазон читання
		log.Printf("Змінено діапазон читання для звіту на: %s", sheetRange)
	} else {
		// Якщо формат неправильний, використовуємо значення за замовчуванням
		sheetRange = "Звіт!A2:E" // Значення за замовчуванням, якщо конфіг невірний
		log.Printf("Використовується діапазон читання за замовчуванням: %s", sheetRange)
	}


	var data SheetRowData
	var err error 

	log.Printf("Спроба читання даних з Google Sheets: SpreadsheetID=%s, Range=%s", spreadsheetID, sheetRange)
	resp, errGet := srv.Spreadsheets.Values.Get(spreadsheetID, sheetRange).Do()
	if errGet != nil {
		log.Printf("Не вдалося отримати дані з Google Sheets (GenerateProgressReport): %v", errGet)
		return data, fmt.Errorf("помилка отримання даних з Google Sheets: %w", errGet)
	}

	// Перевіряємо, чи є хоча б один рядок даних (це буде наш рядок 2)
	if len(resp.Values) < 1 {
		errMsg := fmt.Sprintf("немає даних у діапазоні %s (очікувався хоча б рядок A2)", sheetRange)
		log.Println(errMsg)
		return data, fmt.Errorf(errMsg)
	}

	// Беремо перший рядок з отриманих даних (це має бути рядок 2 аркуша)
	row := resp.Values[0]

	// Перевіряємо, чи в цьому першому рядку є достатньо колонок (A-E)
	if len(row) < 5 {
		errMsg := fmt.Sprintf("недостатньо колонок у першому рядку даних (%s): отримано %d, очікувалося 5", sheetRange, len(row))
		log.Println(errMsg)
		return data, fmt.Errorf(errMsg)
	}


	// Парсинг даних з рядка з поверненням помилки у разі невдачі
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) } else { data.Date = "" }

	if len(row) > 1 {
		incomeStr := fmt.Sprintf("%v", row[1])
		data.Income, err = strconv.ParseFloat(incomeStr, 64)
		if err != nil { log.Printf("Помилка парсингу доходу '%s': %v.", incomeStr, err); return data, fmt.Errorf("некоректне значення Доходу ('%s') в таблиці '%s'", incomeStr, sheetName) }
	} else { return data, fmt.Errorf("відсутнє значення Доходу в таблиці '%s' (колонка B)", sheetName) }

	if len(row) > 2 {
		sheetGoalStr := fmt.Sprintf("%v", row[2])
		data.SheetGoal, err = strconv.ParseFloat(sheetGoalStr, 64)
		if err != nil { log.Printf("Помилка парсингу мети '%s': %v.", sheetGoalStr, err); return data, fmt.Errorf("некоректне значення Мети ('%s') в таблиці '%s'", sheetGoalStr, sheetName) }
	} else { return data, fmt.Errorf("відсутнє значення Мети в таблиці '%s' (колонка C)", sheetName) }

	if len(row) > 3 {
		sheetDaysLeftStr := fmt.Sprintf("%v", row[3])
		data.SheetDaysLeft, err = strconv.Atoi(sheetDaysLeftStr)
		if err != nil { log.Printf("Помилка парсингу днів '%s': %v.", sheetDaysLeftStr, err); return data, fmt.Errorf("некоректний формат 'Днів залишилося' ('%s') в таблиці '%s'", sheetDaysLeftStr, sheetName) }
	} else { return data, fmt.Errorf("відсутнє значення 'Днів залишилося' в таблиці '%s' (колонка D)", sheetName) }

	if len(row) > 4 {
		sheetReqDailyStr := fmt.Sprintf("%v", row[4])
		data.SheetReqDaily, err = strconv.ParseFloat(sheetReqDailyStr, 64)
		if err != nil { log.Printf("Помилка парсингу потр. щодня '%s': %v.", sheetReqDailyStr, err); return data, fmt.Errorf("некоректний формат 'Потрібно щодня' ('%s') в таблиці '%s'", sheetReqDailyStr, sheetName) }
	} else { return data, fmt.Errorf("відсутнє значення 'Потрібно щодня' в таблиці '%s' (колонка E)", sheetName) }

	log.Printf("Дані з аркуша '%s' (рядок 2) успішно розпарсені: %+v", sheetName, data)
	return data, nil // Повертаємо дані та відсутність помилки
}

// AddGoalToSheet ... (код без змін) ...
func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, goalData FinancialGoalData) error { /*...*/ return nil }
// UpdateGoalStatusInSheet ... (код без змін) ...
func UpdateGoalStatusInSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, newStatus string, closedDate time.Time) error { /*...*/ return nil }
// --- Функції для роботи з аркушем "РобочийГрафік" ---
const workLogSheetNameDefault = "РобочийГрафік"
// LogWorkStart ... (код без змін) ...
func LogWorkStart(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, startTime time.Time) error { /*...*/ return nil }
// LogWorkStop ... (код без змін) ...
func LogWorkStop(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, endTime time.Time) (time.Duration, error) { /*...*/ return time.Duration(0), nil }
// LogDayOff ... (код без змін) ...
func LogDayOff(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, dateToLog time.Time) error { /*...*/ return nil }
// GetActiveGoalFromSheet ... (код без змін) ...
func GetActiveGoalFromSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64) (FinancialGoalData, bool, error) { /*...*/ return FinancialGoalData{}, false, nil }
// CountWorkingDaysInRange ... (код без змін) ...
func CountWorkingDaysInRange(srv *sheets.Service, spreadsheetID string, workLogSheetName string, startDate, endDate time.Time) (int, error) { /*...*/ return 0, nil }

// FormatDuration ... (код без змін) ...
func FormatDuration(d time.Duration) string { /*...*/ return "" }

/* // Закоментовано getMotivation
func getMotivation() string { ... }
*/
