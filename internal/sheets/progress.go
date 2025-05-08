package sheets

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/sheets/v4"
)

// Дозвіл на читання та запис
const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets"

// ---- Структури ----
type SheetRowData struct { Date string; Income float64; SheetGoal float64; SheetDaysLeft int; SheetReqDaily float64 }
type FinancialGoalData struct { Amount float64; Currency string; Days int; OriginalText string; SetDate time.Time }
type InvestmentData struct { Type string; Name string; AmountInvested float64; Currency string; DateInvested time.Time }

// ---- Часова зона ----
var KyivLocation *time.Location
func init() { loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Крит. помилка: не завантажено 'Europe/Kyiv': %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv завантажено (sheets).") } }

// ---- Допоміжні функції ----
func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) (int, []interface{}) { 
	if len(sheetData) == 0 { return -1, nil }; startRowIndex := 0
	if len(sheetData) > 0 && len(sheetData[0]) > 0 && (fmt.Sprintf("%v", sheetData[0][0]) == "Дата" || fmt.Sprintf("%v", sheetData[0][0]) == "Date" || fmt.Sprintf("%v", sheetData[0][0]) == "ChatID" || fmt.Sprintf("%v", sheetData[0][0]) == "ID чату користувача") { startRowIndex = 1 }
	for i := startRowIndex; i < len(sheetData); i++ { row := sheetData[i]; if len(row) > 0 { if fmt.Sprintf("%v", row[0]) == dateToFind { return i, row } } }; return -1, nil
}
func FormatDuration(d time.Duration) string { d = d.Round(time.Minute); h := d / time.Hour; d -= h * time.Hour; m := d / time.Minute; return fmt.Sprintf("%dh %dm", h, m) } 

// --- Функції роботи з Google Sheets API ---
func NewService(credentialsJSON []byte) (*sheets.Service, error) { return nil, fmt.Errorf("функція NewService не реалізована") } 

func GenerateProgressReport(srv *sheets.Service, spreadsheetID string, reportSheetNameAndRange string) (SheetRowData, error) {
	var data SheetRowData; var err error; sheetName := "Звіт"; sheetRangeForRead := "A2:E"
	parts := strings.Split(reportSheetNameAndRange, "!"); if len(parts) == 2 && parts[0] != "" && parts[1] != "" { sheetName = parts[0]; sheetRangeForRead = fmt.Sprintf("%s!A2:E", sheetName) } else { sheetRangeForRead = "Звіт!A2:E"; log.Printf("ПОПЕРЕДЖЕННЯ: Некор. формат '%s'. Використ. '%s'", reportSheetNameAndRange, sheetRangeForRead) }
	log.Printf("Спроба читання: SpreadsheetID=%s, Range=%s", spreadsheetID, sheetRangeForRead); resp, errGet := srv.Spreadsheets.Values.Get(spreadsheetID, sheetRangeForRead).Do(); if errGet != nil { return data, fmt.Errorf("помилка GSheets(Report): %w", errGet) }
	if len(resp.Values) < 1 { return data, fmt.Errorf("немає даних у '%s'", sheetRangeForRead) }
	row := resp.Values[0]; if len(row) < 5 { return data, fmt.Errorf("мало колонок у %s (рядок 2)", sheetName) }
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) }
	if len(row) > 1 { incomeStr := fmt.Sprintf("%v", row[1]); data.Income, err = strconv.ParseFloat(incomeStr, 64); if err != nil { return data, fmt.Errorf("некор. Дохід ('%s') в '%s'", incomeStr, sheetName) } } else { return data, fmt.Errorf("відсутній Дохід в '%s' (B)", sheetName) }
	if len(row) > 2 { sheetGoalStr := fmt.Sprintf("%v", row[2]); data.SheetGoal, err = strconv.ParseFloat(sheetGoalStr, 64); if err != nil { return data, fmt.Errorf("некор. Мета ('%s') в '%s'", sheetGoalStr, sheetName) } } else { return data, fmt.Errorf("відсутня Мета в '%s' (C)", sheetName) }
	if len(row) > 3 { sheetDaysLeftStr := fmt.Sprintf("%v", row[3]); data.SheetDaysLeft, err = strconv.Atoi(sheetDaysLeftStr); if err != nil { return data, fmt.Errorf("некор. 'Дні Зал.' ('%s') в '%s'", sheetDaysLeftStr, sheetName) } } else { return data, fmt.Errorf("відсутні 'Дні Зал.' в '%s' (D)", sheetName) }
	if len(row) > 4 { sheetReqDailyStr := fmt.Sprintf("%v", row[4]); data.SheetReqDaily, err = strconv.ParseFloat(sheetReqDailyStr, 64); if err != nil { return data, fmt.Errorf("некор. 'Потр. Щодня' ('%s') в '%s'", sheetReqDailyStr, sheetName) } } else { return data, fmt.Errorf("відсутнє 'Потр. Щодня' в '%s' (E)", sheetName) }
	log.Printf("Дані з аркуша '%s!A2:E...' успішно розпарсені: %+v", sheetName, data); return data, nil
}

func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, goalData FinancialGoalData) error { 
	log.Printf("Додавання цілі на '%s' ChatID %d: %+v", goalsSheetName, chatID, goalData); var rowValues []interface{}; rowValues = append(rowValues, chatID, goalData.Amount, goalData.Currency, 0 /*Days=0*/, goalData.SetDate.In(KyivLocation).Format("2006-01-02"), "Активна", goalData.OriginalText, ""); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowValues}}; appendRange := fmt.Sprintf("%s!A:H", goalsSheetName); _, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do(); if err != nil { log.Printf("Помилка запису цілі '%s': %v", goalsSheetName, err); return fmt.Errorf("запис цілі: %w", err) }; log.Printf("Ціль ChatID %d записана '%s'", chatID, goalsSheetName); return nil
}

func UpdateGoalStatusInSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, newStatus string, closedDate time.Time) error { 
	log.Printf("Оновлення статусу '%s' на '%s' ChatID %d", newStatus, goalsSheetName, chatID); readRange := fmt.Sprintf("%s!A:F", goalsSheetName); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); if err != nil { return fmt.Errorf("читання для оновл.: %w", err) }; targetSheetRowIndex := -1; if len(resp.Values) > 0 { for i := len(resp.Values) - 1; i >= 0; i-- { row := resp.Values[i]; isHeader := (i == 0 && len(row) > 0 && (fmt.Sprintf("%v", row[0]) == "ChatID" || fmt.Sprintf("%v", row[0]) == "ID чату користувача")); if isHeader || len(row) < 6 { continue }; rowChatIDStr := fmt.Sprintf("%v", row[0]); rowStatusStr := fmt.Sprintf("%v", row[5]); rowChatID, errChatID := strconv.ParseInt(rowChatIDStr, 10, 64); if errChatID == nil && rowChatID == chatID && rowStatusStr == "Активна" { targetSheetRowIndex = i + 1; break } } }; if targetSheetRowIndex == -1 { return fmt.Errorf("не знайдено активної цілі") }; statusUpdateRange := fmt.Sprintf("%s!F%d", goalsSheetName, targetSheetRowIndex); statusValueRange := &sheets.ValueRange{Values: [][]interface{}{{newStatus}}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { return fmt.Errorf("оновлення статусу: %w", err) }; closedDateUpdateRange := fmt.Sprintf("%s!H%d", goalsSheetName, targetSheetRowIndex); closedDateValueRange := &sheets.ValueRange{Values: [][]interface{}{{closedDate.In(KyivLocation).Format("2006-01-02")}}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, closedDateUpdateRange, closedDateValueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { log.Printf("Помилка оновл. дати закриття: %v", err) }; log.Printf("Статус цілі ChatID %d (рядок %d) '%s' оновлено на '%s'", chatID, targetSheetRowIndex, goalsSheetName, newStatus); return nil
}

// !!! ПОВНЕ ТІЛО ФУНКЦІЇ AddInvestmentToSheet !!!
func AddInvestmentToSheet(srv *sheets.Service, spreadsheetID string, investmentsSheetName string, chatID int64, invData InvestmentData) error {
	log.Printf("Додавання інвестиції на аркуш '%s' для ChatID %d: %+v", investmentsSheetName, chatID, invData)
	// A: ID, B: Тип, C: Назва, D: Сума вкл., E: Валюта вкл., F: Дата вкл., G: Пот. вартість, H: Приб/Збит, I: Статус, J: Ціль приб., K: Нотатки
	var rowValues []interface{}
	rowValues = append(rowValues,
		"", // ID - поки що порожньо
		invData.Type,
		invData.Name,
		invData.AmountInvested,
		invData.Currency,
		invData.DateInvested.In(KyivLocation).Format("2006-01-02"), // Дата в локальному форматі
		"", // G: Пот. вартість
		"", // H: Приб/Збит
		"Активна", // I: Статус
		"", // J: Ціль приб.
		"", // K: Нотатки
	)
	valueRange := &sheets.ValueRange{Values: [][]interface{}{rowValues}}
	appendRange := fmt.Sprintf("%s!A:K", investmentsSheetName) 
	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
		ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()
	if err != nil { log.Printf("Помилка запису інвестиції '%s': %v", investmentsSheetName, err); return fmt.Errorf("не вдалося записати інвестицію: %w", err) }
	log.Printf("Інвестиція для ChatID %d записана на '%s'", chatID, investmentsSheetName); return nil
}

// --- Функції для роботи з аркушем "РобочийГрафік" ---
const workLogSheetNameDefault = "РобочийГрафік"

func LogWorkStart(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, startTime time.Time) error { /* ... повний код з #143 ... */ return nil }
func LogWorkStop(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, endTime time.Time) (time.Duration, error) { /* ... повний код з #143 ... */ return time.Duration(0), nil }
func LogDayOff(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, dateToLog time.Time) error { /* ... повний код з #143 ... */ return nil }
func GetActiveGoalFromSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64) (FinancialGoalData, bool, error) { /* ... повний код з #143 ... */ return FinancialGoalData{}, false, nil }
func CountWorkingDaysInRange(srv *sheets.Service, spreadsheetID string, workLogSheetName string, startDate, endDate time.Time) (int, error) { /* ... повний код з #143 ... */ return 0, nil }
