package sheets

import (
	"fmt"
	"log"
	"strconv"
	"strings" 
	"time"

	"google.golang.org/api/sheets/v4"
)

const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets"

// ---- Структури ----
type SheetRowData struct { Date string; Income float64; SheetGoal float64; SheetDaysLeft int; SheetReqDaily float64 }
type FinancialGoalData struct { Amount float64; Currency string; Days int; OriginalText string; SetDate time.Time }

// ---- Часова зона ----
var KyivLocation *time.Location
func init() { loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Крит. помилка: не завантажено 'Europe/Kyiv': %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv завантажено (sheets).") } }
// GetCurrentTimeInKyiv можна зробити публічною, якщо треба
// func GetCurrentTimeInKyiv() time.Time { return time.Now().In(KyivLocation) }

// ---- Допоміжні функції ----
// Функцію findRowByDate ВИДАЛЕНО

// FormatDuration публічна функція для форматування тривалості
func FormatDuration(d time.Duration) string { 
	d = d.Round(time.Minute); h := d / time.Hour; d -= h * time.Hour; m := d / time.Minute; return fmt.Sprintf("%dh %dm", h, m) 
}

// --- Функції роботи з Google Sheets API ---
func NewService(credentialsJSON []byte) (*sheets.Service, error) { return nil, fmt.Errorf("функція NewService не реалізована") } 

// GenerateProgressReport - без змін з попередньої версії
func GenerateProgressReport(srv *sheets.Service, spreadsheetID string, reportSheetNameAndRange string) (SheetRowData, error) { /* ... код без змін з #171 ... */ return SheetRowData{}, nil }
// AddGoalToSheet - без змін з попередньої версії
func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, goalData FinancialGoalData) error { /* ... код без змін з #143 ... */ return nil }
// UpdateGoalStatusInSheet - без змін з попередньої версії
func UpdateGoalStatusInSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, newStatus string, closedDate time.Time) error { /* ... код без змін з #143 ... */ return nil }

// --- Функції для роботи з аркушем "РобочийГрафік" ---
const workLogSheetNameDefault = "РобочийГрафік"

// LogWorkStart - логіка пошуку рядка тепер всередині
func LogWorkStart(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, startTime time.Time) error {
	if workLogSheetName == "" { workLogSheetName = workLogSheetNameDefault }
	localStartTime := startTime.In(KyivLocation)
	todayStr := localStartTime.Format("2006-01-02")
	startTimeStr := localStartTime.Format("15:04:05")
	log.Printf("Лог старту: ChatID=%d, Date=%s, Time=%s, Sheet=%s", chatID, todayStr, startTimeStr, workLogSheetName)

	// Шукаємо рядок для сьогоднішньої дати
	actualSheetRowIndex := -1 // 1-based index
	readRange := fmt.Sprintf("%s!A:A", workLogSheetName)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil { log.Printf("Помилка читання дат '%s': %v.", workLogSheetName, err) } else {
		// Логіка пошуку рядка (раніше була в findRowByDate)
		sheetData := resp.Values
		if len(sheetData) > 0 {
			startRowIndex := 0
			if len(sheetData[0]) > 0 && (fmt.Sprintf("%v", sheetData[0][0]) == "Дата" || fmt.Sprintf("%v", sheetData[0][0]) == "Date") { startRowIndex = 1 }
			for i := startRowIndex; i < len(sheetData); i++ {
				row := sheetData[i]
				if len(row) > 0 { if fmt.Sprintf("%v", row[0]) == todayStr { actualSheetRowIndex = i + 1; break } }
			}
		}
	}

	rowData := []interface{}{todayStr, "Розпочато", startTimeStr, "", ""}
	if actualSheetRowIndex != -1 { // Якщо рядок знайдено, оновлюємо
		log.Printf("Знайдено рядок %d для дати %s. Оновлення...", actualSheetRowIndex, todayStr)
		updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, actualSheetRowIndex, actualSheetRowIndex)
		valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}; _, errUpdate := srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).ValueInputOption("USER_ENTERED").Do()
		if errUpdate != nil { return fmt.Errorf("оновл. старту: %w", errUpdate) }; log.Printf("Рядок %d (%s) оновлено (старт).", actualSheetRowIndex, todayStr)
	} else { // Якщо рядок не знайдено, додаємо новий
		log.Printf("Не знайдено рядка для дати %s. Додавання нового...", todayStr)
		appendRange := fmt.Sprintf("%s!A:E", workLogSheetName)
		valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}; _, errAppend := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()
		if errAppend != nil { return fmt.Errorf("додавання старту: %w", errAppend) }; log.Printf("Новий рядок %s додано (старт).", todayStr)
	}
	return nil
}

// LogWorkStop - логіка пошуку рядка тепер всередині
func LogWorkStop(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, endTime time.Time) (time.Duration, error) {
	if workLogSheetName == "" { workLogSheetName = workLogSheetNameDefault }; localEndTime := endTime.In(KyivLocation); todayStr := localEndTime.Format("2006-01-02"); endTimeStr := localEndTime.Format("15:04:05"); log.Printf("Лог стоп: ChatID=%d, Date=%s, Time=%s, Sheet=%s", chatID, todayStr, endTimeStr, workLogSheetName); zeroDuration := time.Duration(0); 
	
	// Шукаємо рядок для сьогоднішньої дати
	actualSheetRowIndex := -1 // 1-based index
	var rowDataFromFind []interface{} // Зберігаємо знайдений рядок
	readRange := fmt.Sprintf("%s!A:C", workLogSheetName) // Читаємо A, B, C
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); 
	if err != nil { return zeroDuration, fmt.Errorf("читання для стоп: %w", err) } else {
		// Логіка пошуку рядка (раніше була в findRowByDate)
		sheetData := resp.Values
		if len(sheetData) > 0 {
			startRowIndex := 0
			if len(sheetData[0]) > 0 && (fmt.Sprintf("%v", sheetData[0][0]) == "Дата" || fmt.Sprintf("%v", sheetData[0][0]) == "Date") { startRowIndex = 1 }
			// Шукаємо ЗНИЗУ ВГОРУ, щоб знайти останній запис за сьогодні, якщо їх декілька
			for i := len(sheetData) - 1; i >= startRowIndex; i-- {
				row := sheetData[i]
				if len(row) > 0 { if fmt.Sprintf("%v", row[0]) == todayStr { actualSheetRowIndex = i + 1; rowDataFromFind = row; break } }
			}
		}
	}

	if actualSheetRowIndex == -1 { return zeroDuration, fmt.Errorf("не знайдено запис про старт") }; 
	
	// Розрахунок тривалості
	var startTimeInKyiv time.Time; var duration time.Duration = zeroDuration; var currentStatus string = "Невідомо"; 
	if len(rowDataFromFind) >= 3 { startTimeSheetStr := fmt.Sprintf("%v", rowDataFromFind[2]); currentStatus = fmt.Sprintf("%v", rowDataFromFind[1]); parsedStartTime, errTime := time.ParseInLocation("15:04:05", startTimeSheetStr, KyivLocation); if errTime == nil { year, month, day := localEndTime.Date(); startTimeInKyiv = time.Date(year, month, day, parsedStartTime.Hour(), parsedStartTime.Minute(), parsedStartTime.Second(), 0, KyivLocation); if (currentStatus == "Розпочато" || currentStatus == "Робочий") && localEndTime.After(startTimeInKyiv) { duration = localEndTime.Sub(startTimeInKyiv) } else { log.Printf("Статус не 'Розпочато' для %d (%s).", chatID, currentStatus) } } else { log.Printf("Не розпарсено час '%s': %v", startTimeSheetStr, errTime) } } else { log.Printf("Мало даних у рядку %d.", actualSheetRowIndex) }; 
	
	// Оновлення таблиці
	durationStr := FormatDuration(duration); newStatus := "Завершено"; updateRangeDE := fmt.Sprintf("%s!D%d:E%d", workLogSheetName, actualSheetRowIndex, actualSheetRowIndex); valueRangeDE := &sheets.ValueRange{Values: [][]interface{}{{endTimeStr, durationStr}}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRangeDE, valueRangeDE).ValueInputOption("USER_ENTERED").Do(); if err != nil { log.Printf("Помилка оновл. D:E %d: %v", actualSheetRowIndex, err) }; statusUpdateRange := fmt.Sprintf("%s!B%d", workLogSheetName, actualSheetRowIndex); statusValueRange := &sheets.ValueRange{Values: [][]interface{}{{newStatus}}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { return zeroDuration, fmt.Errorf("оновл. стоп запису: %w", err) }; log.Printf("Рядок %d (%s) оновлено (завершено, %s).", actualSheetRowIndex, todayStr, durationStr); return duration, nil
}

// LogDayOff - логіка пошуку рядка тепер всередині
func LogDayOff(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, dateToLog time.Time) error {
	if workLogSheetName == "" { workLogSheetName = workLogSheetNameDefault }; localDate := dateToLog.In(KyivLocation); dateStr := localDate.Format("2006-01-02"); log.Printf("Лог вихідного: ChatID=%d, Date=%s, Sheet=%s", chatID, dateStr, workLogSheetName); 
	
	// Шукаємо рядок для дати
	actualSheetRowIndex := -1 // 1-based index
	readRange := fmt.Sprintf("%s!A:A", workLogSheetName); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); var sheetValues [][]interface{}; if err != nil { log.Printf("Помилка читання дат '%s': %v.", workLogSheetName, err) } else { sheetValues = resp.Values }; 
	if len(sheetValues) > 0 { // Використовуємо sheetValues, а не resp.Values напряму
		startRowIndex := 0; if len(sheetValues[0]) > 0 && (fmt.Sprintf("%v", sheetValues[0][0]) == "Дата" || fmt.Sprintf("%v", sheetValues[0][0]) == "Date") { startRowIndex = 1 }
		for i := startRowIndex; i < len(sheetValues); i++ { row := sheetValues[i]; if len(row) > 0 { if fmt.Sprintf("%v", row[0]) == dateStr { actualSheetRowIndex = i + 1; break } } }
	}

	rowData := []interface{}{dateStr, "Вихідний", "", "", ""}; 
	if actualSheetRowIndex != -1 {
		log.Printf("Знайдено рядок %d для дати %s. Оновлення...", actualSheetRowIndex, dateStr)
		updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, actualSheetRowIndex, actualSheetRowIndex); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { return fmt.Errorf("оновл. вихідного: %w", err) }; log.Printf("Рядок %d (%s) оновлено (вихідний).", actualSheetRowIndex, dateStr) 
	} else {
		log.Printf("Не знайдено рядка для дати %s. Додавання нового...", dateStr)
		appendRange := fmt.Sprintf("%s!A:E", workLogSheetName); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}; _, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do(); if err != nil { return fmt.Errorf("додавання вихідного: %w", err) }; log.Printf("Новий рядок %s додано (вихідний).", dateStr) 
	}
	return nil
}

// GetActiveGoalFromSheet ... (код без змін з #143/151) ...
func GetActiveGoalFromSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64) (FinancialGoalData, bool, error) { /* ... */ return FinancialGoalData{}, false, nil }
// CountWorkingDaysInRange ... (код без змін з #143/151) ...
func CountWorkingDaysInRange(srv *sheets.Service, spreadsheetID string, workLogSheetName string, startDate, endDate time.Time) (int, error) { /* ... */ return 0, nil }
