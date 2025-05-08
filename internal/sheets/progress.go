package sheets

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets"

// ---- Структури ----
type SheetRowData struct { Date string; Income float64; SheetGoal float64; SheetDaysLeft int; SheetReqDaily float64 }
type FinancialGoalData struct { Amount float64; Currency string; Days int; OriginalText string; SetDate time.Time }

// ---- Часова зона ----
var KyivLocation *time.Location
func init() { loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Критична помилка: не вдалося завантажити 'Europe/Kyiv': %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv успішно завантажено (пакет sheets).") } }
func GetCurrentTimeInKyiv() time.Time { return time.Now().In(KyivLocation) }

// ---- Допоміжні функції ----
func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) (int, []interface{}) { /* ... код без змін ... */ if len(sheetData) == 0 { return -1, nil }; startRowIndex := 0; if len(sheetData) > 0 && len(sheetData[0]) > 0 && (fmt.Sprintf("%v", sheetData[0][0]) == "Дата" || fmt.Sprintf("%v", sheetData[0][0]) == "Date" || fmt.Sprintf("%v", sheetData[0][0]) == "ChatID" || fmt.Sprintf("%v", sheetData[0][0]) == "ID чату користувача") { startRowIndex = 1 }; for i := startRowIndex; i < len(sheetData); i++ { row := sheetData[i]; if len(row) > 0 { if fmt.Sprintf("%v", row[0]) == dateToFind { return i, row } } }; return -1, nil }
func FormatDuration(d time.Duration) string { /* ... код без змін ... */ d = d.Round(time.Minute); h := d / time.Hour; d -= h * time.Hour; m := d / time.Minute; return fmt.Sprintf("%dh %dm", h, m) }

// --- Функції роботи з Google Sheets API ---
func NewService(credentialsJSON []byte) (*sheets.Service, error) { /*...*/ return nil, fmt.Errorf("функція NewService не реалізована") }
func GenerateProgressReport(srv *sheets.Service, spreadsheetID string, reportSheetNameAndRange string) (SheetRowData, error) { /* ... код без змін з #171 ... */ return SheetRowData{}, nil }
func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, goalData FinancialGoalData) error { /* ... код без змін з #143 ... */ return nil }
func UpdateGoalStatusInSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, newStatus string, closedDate time.Time) error { /* ... код без змін з #143 ... */ return nil }
const workLogSheetNameDefault = "РобочийГрафік"
func LogWorkStart(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, startTime time.Time) error { /*...*/ return nil }
func LogWorkStop(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, endTime time.Time) (time.Duration, error) { /*...*/ return time.Duration(0), nil }
func LogDayOff(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, dateToLog time.Time) error { /*...*/ return nil }

// GetActiveGoalFromSheet з ДОДАТКОВИМ ЛОГУВАННЯМ
func GetActiveGoalFromSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64) (FinancialGoalData, bool, error) {
	var goalData FinancialGoalData; var found bool
	readRange := fmt.Sprintf("%s!A:G", goalsSheetName) 
	log.Printf("Пошук активної цілі для ChatID %d на аркуші '%s'", chatID, goalsSheetName)

	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil { 
		log.Printf("Помилка читання даних з аркуша '%s' для пошуку цілі: %v", goalsSheetName, err)
		return goalData, false, fmt.Errorf("не вдалося прочитати дані цілей: %w", err) 
	}

	log.Printf("Отримано %d рядків з аркуша '%s'", len(resp.Values), goalsSheetName) // Лог кількості рядків

	if len(resp.Values) > 1 { 
		for i := len(resp.Values) - 1; i >= 0; i-- { // Шукаємо з кінця
			row := resp.Values[i]
			log.Printf("Перевірка рядка %d (Sheet row %d)", i, i+1) // Лог рядка, що перевіряється

			// Перевірка на заголовок
			if i == 0 {
				headerCheckStr := ""
				if len(row) > 0 { headerCheckStr = fmt.Sprintf("%v", row[0]) }
				if headerCheckStr == "ChatID" || headerCheckStr == "ID чату користувача" {
					log.Printf("Рядок %d: Пропущено заголовок", i+1)
					continue 
				}
			}
				
			if len(row) < 7 { 
				log.Printf("Рядок %d: Пропущено, недостатньо колонок (%d)", i+1, len(row))
				continue // Пропускаємо, якщо колонок менше, ніж потрібно для статусу та ChatID
			} 
			
			rowChatIDStr := fmt.Sprintf("%v", row[0])
			rowStatusStr := fmt.Sprintf("%v", row[5]) // Колонка F - Status
			rowChatID, errChatID := strconv.ParseInt(rowChatIDStr, 10, 64)

			if errChatID != nil {
				log.Printf("Рядок %d: Помилка парсингу ChatID '%s': %v", i+1, rowChatIDStr, errChatID)
				continue // Пропускаємо рядок з невірним ChatID
			}
			
			log.Printf("Рядок %d: Знайдено ChatID=%d, Status='%s'", i+1, rowChatID, rowStatusStr) // Лог даних рядка

			if rowChatID == chatID { // Знайшли рядок для нашого користувача
				log.Printf("Рядок %d: ChatID збігається (%d). Перевірка статусу...", i+1, chatID)
				if rowStatusStr == "Активна" { // Перевіряємо статус
					// Знайшли ОСТАННЮ АКТИВНУ ціль!
					log.Printf("Рядок %d: Знайдено АКТИВНУ ціль для ChatID %d!", i+1, chatID)
					amountStr := fmt.Sprintf("%v", row[1]); goalData.Amount, _ = strconv.ParseFloat(amountStr, 64)
					goalData.Currency = fmt.Sprintf("%v", row[2])
					goalData.Days, _ = strconv.Atoi(fmt.Sprintf("%v", row[3])); setDateStr := fmt.Sprintf("%v", row[4])
					goalData.OriginalText = fmt.Sprintf("%v", row[6])
					parsedSetDate, errDate := time.ParseInLocation("2006-01-02", setDateStr, KyivLocation); 
					if errDate == nil { goalData.SetDate = parsedSetDate.UTC() } else { goalData.SetDate = time.Now().UTC(); log.Printf("Рядок %d: Помилка парсингу SetDate '%s': %v.", i+1, setDateStr, errDate)}
					found = true
					break // Виходимо з циклу, бо знайшли останню активну
				} else {
					log.Printf("Рядок %d: Статус НЕ 'Активна' ('%s'). Продовжуємо пошук.", i+1, rowStatusStr)
				}
			}
		}
	}

	if !found { 
		log.Printf("Активну ціль для ChatID %d на аркуші '%s' НЕ знайдено після перевірки всіх рядків.", chatID, goalsSheetName)
	}
	return goalData, found, nil 
}

func CountWorkingDaysInRange(srv *sheets.Service, spreadsheetID string, workLogSheetName string, startDate, endDate time.Time) (int, error) { /*...*/ return 0, nil }

/* // getMotivation закоментовано
func getMotivation() string { ... }
*/

// ВАЖЛИВО: Переконайтеся, що повні тіла для скорочених функцій вище (позначених /*...*/)
// відповідають коду з відповіді #143. Я скоротив їх тут для читабельності.
