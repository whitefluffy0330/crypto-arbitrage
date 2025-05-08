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

// НОВА СТРУКТУРА для даних інвестиції
type InvestmentData struct {
	Type           string    // Тип (Крипто-холд, DeFi, Акція, ETF, Перепродаж, P2P, Освіта...)
	Name           string    // Назва/Актив (BTC, ETH, AAPL, Курс Go...)
	AmountInvested float64   // Сума вкладення
	Currency       string    // Валюта вкладення (UAH, USD...)
	DateInvested   time.Time // Дата вкладення (в UTC)
	// Поля, які поки що заповнюються вручну або іншими командами:
	// CurrentValue   float64
	// ProfitTarget   float64
	// Status         string // Буде "Активна" за замовчуванням
	// Notes          string
}


// ---- Часова зона ----
var KyivLocation *time.Location
func init() { loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Крит. помилка: не завантажено 'Europe/Kyiv': %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv завантажено (sheets).") } }
func GetCurrentTimeInKyiv() time.Time { return time.Now().In(KyivLocation) }

// ---- Допоміжні функції ----
func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) (int, []interface{}) { /* ... код без змін ... */ return -1, nil }
func FormatDuration(d time.Duration) string { /* ... код без змін ... */ return "" }

// --- Функції роботи з Google Sheets API ---
func NewService(credentialsJSON []byte) (*sheets.Service, error) { /*...*/ return nil, fmt.Errorf("not implemented") }
func GenerateProgressReport(srv *sheets.Service, spreadsheetID string, reportSheetNameAndRange string) (SheetRowData, error) { /* ... код без змін ... */ return SheetRowData{}, nil }
func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, goalData FinancialGoalData) error { /* ... код без змін ... */ return nil }
func UpdateGoalStatusInSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, newStatus string, closedDate time.Time) error { /* ... код без змін ... */ return nil }
const workLogSheetNameDefault = "РобочийГрафік"
func LogWorkStart(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, startTime time.Time) error { /*...*/ return nil } 
func LogWorkStop(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, endTime time.Time) (time.Duration, error) { /*...*/ return time.Duration(0), nil } 
func LogDayOff(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, dateToLog time.Time) error { /*...*/ return nil } 
func GetActiveGoalFromSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64) (FinancialGoalData, bool, error) { /*...*/ return FinancialGoalData{}, false, nil } 
func CountWorkingDaysInRange(srv *sheets.Service, spreadsheetID string, workLogSheetName string, startDate, endDate time.Time) (int, error) { /*...*/ return 0, nil } 


// НОВА ФУНКЦІЯ: AddInvestmentToSheet додає рядок з інвестицією на вказаний аркуш
func AddInvestmentToSheet(srv *sheets.Service, spreadsheetID string, investmentsSheetName string, chatID int64, invData InvestmentData) error {
	log.Printf("Додавання інвестиції на аркуш '%s' для ChatID %d: %+v", investmentsSheetName, chatID, invData)

	// Готуємо рядок даних для запису відповідно до структури аркуша "Інвестиції"
	// A: ID (залишаємо порожнім, або генеруємо), B: Тип, C: Назва, D: Сума вкл., E: Валюта вкл., F: Дата вкл., G: Пот. вартість, H: Приб/Збит, I: Статус, J: Ціль приб., K: Нотатки
	var rowValues []interface{}
	rowValues = append(rowValues,
		"", // ID - поки що порожньо, можна генерувати пізніше
		invData.Type,
		invData.Name,
		invData.AmountInvested,
		invData.Currency,
		invData.DateInvested.In(KyivLocation).Format("2006-01-02"), // Записуємо дату в локальному форматі
		"", // Поточна вартість - порожньо
		"", // Прибуток/Збиток - порожньо або формула
		"Активна", // Нова інвестиція завжди активна
		"", // Ціль прибутку - порожньо
		"", // Нотатки - порожньо
	)

	valueRange := &sheets.ValueRange{Values: [][]interface{}{rowValues}}
	// Додаємо в кінець аркуша, в колонки A-K
	appendRange := fmt.Sprintf("%s!A:K", investmentsSheetName) 

	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
		ValueInputOption("USER_ENTERED"). // Дозволяє Sheets інтерпретувати дані (дати, числа)
		InsertDataOption("INSERT_ROWS").  // Вставляє новий рядок
		Do()

	if err != nil {
		log.Printf("Помилка запису інвестиції на аркуш '%s' для ChatID %d: %v", investmentsSheetName, chatID, err)
		return fmt.Errorf("не вдалося записати інвестицію у таблицю: %w", err)
	}

	log.Printf("Інвестицію для ChatID %d успішно записано на аркуш '%s'", chatID, investmentsSheetName)
	return nil
}

// Скорочені тіла попередніх функцій (залишайте їх повними у вашому файлі!)
// ... (GenerateProgressReport, AddGoalToSheet, UpdateGoalStatusInSheet, LogWorkStart, LogWorkStop, LogDayOff, GetActiveGoalFromSheet, CountWorkingDaysInRange) ...
