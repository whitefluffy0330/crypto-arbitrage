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
func init() { loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Критична помилка: не завантажено 'Europe/Kyiv': %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv завантажено (sheets).") } }
func GetCurrentTimeInKyiv() time.Time { return time.Now().In(KyivLocation) }

// ---- Допоміжні функції ----
func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) (int, []interface{}) { /* ... */ return -1, nil } // Скорочено для читабельності, код той самий
func FormatDuration(d time.Duration) string { /* ... */ return "" } // Скорочено для читабельності, код той самий

// --- Функції роботи з Google Sheets API ---
func NewService(credentialsJSON []byte) (*sheets.Service, error) { return nil, fmt.Errorf("не реалізовано") }

func GenerateProgressReport(srv *sheets.Service, spreadsheetID string, reportSheetNameAndRange string) (SheetRowData, error) { /* ... код без змін з #171 ... */ return SheetRowData{}, nil }

// AddGoalToSheet тепер записує 0 у колонку Days
func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, goalData FinancialGoalData) error {
	log.Printf("Додавання цілі на аркуш '%s' для ChatID %d: %+v", goalsSheetName, chatID, goalData)
	
	// A: ChatID, B: Amount, C: Currency, D: Days, E: SetDate, F: Status, G: OriginalText, H: ClosedDate
	var rowValues []interface{}
	rowValues = append(rowValues, 
		chatID,                    // Колонка A
		goalData.Amount,           // Колонка B
		goalData.Currency,         // Колонка C
		0,                         // <<< Колонка D (Days): ЗАПИСУЄМО 0, оскільки ціль місячна
		goalData.SetDate.In(KyivLocation).Format("2006-01-02"), // Колонка E
		"Активна",                 // Колонка F
		goalData.OriginalText,     // Колонка G
		"",                        // Колонка H
	) 

	valueRange := &sheets.ValueRange{Values: [][]interface{}{rowValues}}
	appendRange := fmt.Sprintf("%s!A:H", goalsSheetName) 

	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
		ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()

	if err != nil { log.Printf("Помилка запису цілі на '%s': %v", goalsSheetName, err); return fmt.Errorf("не вдалося записати ціль: %w", err) }
	log.Printf("Ціль для ChatID %d записана на '%s'", chatID, goalsSheetName); return nil
}

// UpdateGoalStatusInSheet ... (код без змін з #143/151) ...
func UpdateGoalStatusInSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, newStatus string, closedDate time.Time) error { /* ... */ return nil }
const workLogSheetNameDefault = "РобочийГрафік"
func LogWorkStart(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, startTime time.Time) error { /*...*/ return nil } 
func LogWorkStop(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, endTime time.Time) (time.Duration, error) { /*...*/ return time.Duration(0), nil } 
func LogDayOff(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, dateToLog time.Time) error { /*...*/ return nil } 
func GetActiveGoalFromSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64) (FinancialGoalData, bool, error) { /*...*/ return FinancialGoalData{}, false, nil } 
func CountWorkingDaysInRange(srv *sheets.Service, spreadsheetID string, workLogSheetName string, startDate, endDate time.Time) (int, error) { /*...*/ return 0, nil }
