package sheets

import (
	// "context" // ВИДАЛЕНО
	"fmt"
	"log"
	"strconv"
	"strings" // ПОТРІБЕН для strings.Split
	"time"

	// "google.golang.org/api/option" // ВИДАЛЕНО
	"google.golang.org/api/sheets/v4"
)

const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets"

// ---- Структури ----
type SheetRowData struct { Date string; Income float64; SheetGoal float64; SheetDaysLeft int; SheetReqDaily float64 }
type FinancialGoalData struct { Amount float64; Currency string; Days int; OriginalText string; SetDate time.Time }

// ---- Часова зона ----
var KyivLocation *time.Location
func init() { loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Критична помилка: не завантажено 'Europe/Kyiv': %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv завантажено (sheets).") } }
func GetCurrentTimeInKyiv() time.Time { return time.Now().In(KyivLocation) }

// ---- Допоміжні функції ----
func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) (int, []interface{}) { if len(sheetData) == 0 { return -1, nil }; startRowIndex := 0; if len(sheetData) > 0 && len(sheetData[0]) > 0 && (fmt.Sprintf("%v", sheetData[0][0]) == "Дата" || fmt.Sprintf("%v", sheetData[0][0]) == "Date" || fmt.Sprintf("%v", sheetData[0][0]) == "ChatID" || fmt.Sprintf("%v", sheetData[0][0]) == "ID чату користувача") { startRowIndex = 1 }; for i := startRowIndex; i < len(sheetData); i++ { row := sheetData[i]; if len(row) > 0 { if fmt.Sprintf("%v", row[0]) == dateToFind { return i, row } } }; return -1, nil }
func FormatDuration(d time.Duration) string { d = d.Round(time.Minute); h := d / time.Hour; d -= h * time.Hour; m := d / time.Minute; return fmt.Sprintf("%dh %dm", h, m) }

// --- Функції роботи з Google Sheets API ---
func NewService(credentialsJSON []byte) (*sheets.Service, error) { return nil, fmt.Errorf("функція NewService не реалізована") }

func GenerateProgressReport(srv *sheets.Service, spreadsheetID string, reportSheetNameAndRange string) (SheetRowData, error) {
	var data SheetRowData; var err error
	sheetName := "Звіт"; sheetRangeForRead := "A2:E"
	parts := strings.Split(reportSheetNameAndRange, "!") // ВИКОРИСТАННЯ strings
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" { sheetName = parts[0]; sheetRangeForRead = fmt.Sprintf("%s!A2:E", sheetName) } else { sheetRangeForRead = "Звіт!A2:E"; log.Printf("ПОПЕРЕДЖЕННЯ: Некоректний формат '%s'. Використ. '%s'", reportSheetNameAndRange, sheetRangeForRead) }
	log.Printf("Спроба читання: SpreadsheetID=%s, Range=%s", spreadsheetID, sheetRangeForRead)
	resp, errGet := srv.Spreadsheets.Values.Get(spreadsheetID, sheetRangeForRead).Do(); if errGet != nil { return data, fmt.Errorf("помилка GSheets(Report): %w", errGet) }
	if len(resp.Values) < 1 { return data, fmt.Errorf("немає даних у '%s'", sheetRangeForRead) }
	row := resp.Values[0]; if len(row) < 5 { return data, fmt.Errorf("мало колонок у %s (рядок 2)", sheetName) }
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) }
	if len(row) > 1 { incomeStr := fmt.Sprintf("%v", row[1]); data.Income, err = strconv.ParseFloat(incomeStr, 64); if err != nil { return data, fmt.Errorf("некор. Дохід ('%s') в '%s'", incomeStr, sheetName) } } else { return data, fmt.Errorf("відсутній Дохід в '%s' (B)", sheetName) }
	if len(row) > 2 { sheetGoalStr := fmt.Sprintf("%v", row[2]); data.SheetGoal, err = strconv.ParseFloat(sheetGoalStr, 64); if err != nil { return data, fmt.Errorf("некор. Мета ('%s') в '%s'", sheetGoalStr, sheetName) } } else { return data, fmt.Errorf("відсутня Мета в '%s' (C)", sheetName) }
	if len(row) > 3 { sheetDaysLeftStr := fmt.Sprintf("%v", row[3]); data.SheetDaysLeft, err = strconv.Atoi(sheetDaysLeftStr); if err != nil { return data, fmt.Errorf("некор. 'Дні Зал.' ('%s') в '%s'", sheetDaysLeftStr, sheetName) } } else { return data, fmt.Errorf("відсутні 'Дні Зал.' в '%s' (D)", sheetName) }
	if len(row) > 4 { sheetReqDailyStr := fmt.Sprintf("%v", row[4]); data.SheetReqDaily, err = strconv.ParseFloat(sheetReqDailyStr, 64); if err != nil { return data, fmt.Errorf("некор. 'Потр. Щодня' ('%s') в '%s'", sheetReqDailyStr, sheetName) } } else { return data, fmt.Errorf("відсутнє 'Потр. Щодня' в '%s' (E)", sheetName) }
	log.Printf("Дані з аркуша '%s!A2:E2' розпарсені: %+v", sheetName, data); return data, nil
}

func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, goalData FinancialGoalData) error { /* ... код без змін ... */ return nil }
func UpdateGoalStatusInSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, newStatus string, closedDate time.Time) error { /* ... код без змін ... */ return nil }
const workLogSheetNameDefault = "РобочийГрафік"
func LogWorkStart(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, startTime time.Time) error { /*...*/ return nil }
func LogWorkStop(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, endTime time.Time) (time.Duration, error) { /*...*/ return time.Duration(0), nil }
func LogDayOff(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, dateToLog time.Time) error { /*...*/ return nil }
func GetActiveGoalFromSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64) (FinancialGoalData, bool, error) { /*...*/ return FinancialGoalData{}, false, nil }
func CountWorkingDaysInRange(srv *sheets.Service, spreadsheetID string, workLogSheetName string, startDate, endDate time.Time) (int, error) { /*...*/ return 0, nil }

// Функції AddGoalToSheet, UpdateGoalStatusInSheet, LogWorkStart, LogWorkStop, LogDayOff, GetActiveGoalFromSheet, CountWorkingDaysInRange
// залишаються такими ж, як у відповіді #143. Я скоротив їх тут для читабельності, але ви використовуйте повні версії звідти.
// Переконайтеся, що getMotivation() закоментована або видалена, якщо не використовується тут.
