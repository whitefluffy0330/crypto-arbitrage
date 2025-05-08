package sheets

import (
	"fmt"
	"log"
	"strconv"
	"strings" // Потрібен
	"time"

	"google.golang.org/api/sheets/v4"
)

const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets"

// ---- Структури ----
type SheetRowData struct { Date string; Income float64; SheetGoal float64; SheetDaysLeft int; SheetReqDaily float64 }
type FinancialGoalData struct { Amount float64; Currency string; Days int; OriginalText string; SetDate time.Time }
// НОВА СТРУКТУРА для даних інвестиції
type InvestmentData struct {
	Type           string    
	Name           string    
	AmountInvested float64   
	Currency       string    
	DateInvested   time.Time // В UTC
}

// ---- Часова зона ----
var KyivLocation *time.Location // Експортована
func init() { loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Крит. помилка: не завантажено 'Europe/Kyiv': %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv завантажено (sheets).") } }

// ---- Допоміжні функції ----
func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) (int, []interface{}) { /* ... */ return -1, nil } // Тут має бути повний код з #161
func FormatDuration(d time.Duration) string { /* ... */ return "" } // Тут має бути повний код з #161

// --- Функції роботи з Google Sheets API ---
func NewService(credentialsJSON []byte) (*sheets.Service, error) { return nil, fmt.Errorf("not implemented") }
func GenerateProgressReport(srv *sheets.Service, spreadsheetID string, reportSheetNameAndRange string) (SheetRowData, error) { /* ... код з #171 ... */ return SheetRowData{}, nil }
// НОВА ФУНКЦІЯ: AddInvestmentToSheet додає рядок з інвестицією
func AddInvestmentToSheet(srv *sheets.Service, spreadsheetID string, investmentsSheetName string, chatID int64, invData InvestmentData) error {
	log.Printf("Додавання інвестиції на аркуш '%s' для ChatID %d: %+v", investmentsSheetName, chatID, invData)
	var rowValues []interface{}; rowValues = append(rowValues, "", invData.Type, invData.Name, invData.AmountInvested, invData.Currency, invData.DateInvested.In(KyivLocation).Format("2006-01-02"), "", "", "Активна", "", ""); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowValues}}; appendRange := fmt.Sprintf("%s!A:K", investmentsSheetName); _, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do(); if err != nil { log.Printf("Помилка запису інвестиції '%s': %v", investmentsSheetName, err); return fmt.Errorf("не вдалося записати: %w", err) }; log.Printf("Інвестиція для ChatID %d записана на '%s'", chatID, investmentsSheetName); return nil
}
// ... (Решта функцій: AddGoalToSheet, UpdateGoalStatusInSheet, LogWorkStart, LogWorkStop, LogDayOff, GetActiveGoalFromSheet, CountWorkingDaysInRange - мають бути повними з версії #161) ...
