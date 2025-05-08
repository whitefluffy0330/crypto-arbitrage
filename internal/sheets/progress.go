package sheets

// Правильні імпорти
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
var KyivLocation *time.Location // Експортована
func init() { loc, err := time.LoadLocation("Europe/Kyiv"); if err != nil { log.Printf("Критична помилка: не завантажено 'Europe/Kyiv': %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv завантажено (sheets).") } }

// ---- Допоміжні функції ----

// findRowByDate знаходить 0-based індекс рядка та сам рядок за датою.
func findRowByDate(sheetData [][]interface{}, dateToFind string) (int, []interface{}) { 
	if len(sheetData) == 0 { return -1, nil }; startRowIndex := 0
	// Пропускаємо заголовок, якщо він є
	if len(sheetData) > 0 && len(sheetData[0]) > 0 && (fmt.Sprintf("%v", sheetData[0][0]) == "Дата" || fmt.Sprintf("%v", sheetData[0][0]) == "Date" || fmt.Sprintf("%v", sheetData[0][0]) == "ChatID" || fmt.Sprintf("%v", sheetData[0][0]) == "ID чату користувача") { 
		startRowIndex = 1 
	}
	for i := startRowIndex; i < len(sheetData); i++ { 
		row := sheetData[i]
		if len(row) > 0 { 
			if fmt.Sprintf("%v", row[0]) == dateToFind { 
				return i, row // Повертаємо 0-based індекс та рядок
			} 
		} 
	}
	return -1, nil // Не знайдено
}

// FormatDuration публічна функція для форматування тривалості.
func FormatDuration(d time.Duration) string { 
	d = d.Round(time.Minute); h := d / time.Hour; d -= h * time.Hour; m := d / time.Minute; return fmt.Sprintf("%dh %dm", h, m) 
}

// --- Функції роботи з Google Sheets API ---

// NewService (заглушка, не використовується).
func NewService(credentialsJSON []byte) (*sheets.Service, error) { 
	return nil, fmt.Errorf("функція NewService не реалізована") 
}

// GenerateProgressReport читає дані з аркуша "Звіт" та повертає помилку при некоректних даних.
func GenerateProgressReport(srv *sheets.Service, spreadsheetID string, reportSheetNameAndRange string) (SheetRowData, error) {
	var data SheetRowData; var err error
	sheetName := "Звіт"; sheetRangeForRead := "A2:E"
	
	parts := strings.Split(reportSheetNameAndRange, "!") // ВИКОРИСТАННЯ strings
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" { 
		sheetName = parts[0]
		sheetRangeForRead = fmt.Sprintf("%s!A2:E", sheetName) // Читаємо A2:E
	} else { 
		sheetRangeForRead = "Звіт!A2:E" 
		log.Printf("ПОПЕРЕДЖЕННЯ: Некор. формат '%s'. Використ. '%s'", reportSheetNameAndRange, sheetRangeForRead) 
	}

	log.Printf("Спроба читання: SpreadsheetID=%s, Range=%s", spreadsheetID, sheetRangeForRead)
	resp, errGet := srv.Spreadsheets.Values.Get(spreadsheetID, sheetRangeForRead).Do(); 
	if errGet != nil { return data, fmt.Errorf("помилка GSheets(Report): %w", errGet) }
	if len(resp.Values) < 1 { return data, fmt.Errorf("немає даних у '%s'", sheetRangeForRead) }
	row := resp.Values[0]; if len(row) < 5 { return data, fmt.Errorf("мало колонок у %s (рядок 2)", sheetName) }

	// Парсинг з поверненням помилки (використовуємо strconv)
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) }
	if len(row) > 1 { incomeStr := fmt.Sprintf("%v", row[1]); data.Income, err = strconv.ParseFloat(incomeStr, 64); if err != nil { return data, fmt.Errorf("некор. Дохід ('%s') в '%s'", incomeStr, sheetName) } } else { return data, fmt.Errorf("відсутній Дохід в '%s' (B)", sheetName) }
	if len(row) > 2 { sheetGoalStr := fmt.Sprintf("%v", row[2]); data.SheetGoal, err = strconv.ParseFloat(sheetGoalStr, 64); if err != nil { return data, fmt.Errorf("некор. Мета ('%s') в '%s'", sheetGoalStr, sheetName) } } else { return data, fmt.Errorf("відсутня Мета в '%s' (C)", sheetName) }
	if len(row) > 3 { sheetDaysLeftStr := fmt.Sprintf("%v", row[3]); data.SheetDaysLeft, err = strconv.Atoi(sheetDaysLeftStr); if err != nil { return data, fmt.Errorf("некор. 'Дні Зал.' ('%s') в '%s'", sheetDaysLeftStr, sheetName) } } else { return data, fmt.Errorf("відсутні 'Дні Зал.' в '%s' (D)", sheetName) }
	if len(row) > 4 { sheetReqDailyStr := fmt.Sprintf("%v", row[4]); data.SheetReqDaily, err = strconv.ParseFloat(sheetReqDailyStr, 64); if err != nil { return data, fmt.Errorf("некор. 'Потр. Щодня' ('%s') в '%s'", sheetReqDailyStr, sheetName) } } else { return data, fmt.Errorf("відсутнє 'Потр. Щодня' в '%s' (E)", sheetName) }
	
	log.Printf("Дані з аркуша '%s!A2:E...' успішно розпарсені: %+v", sheetName, data); return data, nil
}

// AddGoalToSheet додає нову ціль на аркуш "МоїЦілі".
func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, goalData FinancialGoalData) error { 
	log.Printf("Додавання цілі на '%s' ChatID %d: %+v", goalsSheetName, chatID, goalData); var rowValues []interface{}; rowValues = append(rowValues, chatID, goalData.Amount, goalData.Currency, 0 /*Days=0*/, goalData.SetDate.In(KyivLocation).Format("2006-01-02"), "Активна", goalData.OriginalText, ""); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowValues}}; appendRange := fmt.Sprintf("%s!A:H", goalsSheetName); _, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do(); if err != nil { log.Printf("Помилка запису цілі '%s': %v", goalsSheetName, err); return fmt.Errorf("запис цілі: %w", err) }; log.Printf("Ціль ChatID %d записана '%s'", chatID, goalsSheetName); return nil
}

// UpdateGoalStatusInSheet знаходить останню активну ціль і оновлює статус.
func UpdateGoalStatusInSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, newStatus string, closedDate time.Time) error { 
	log.Printf("Оновлення статусу '%s' на '%s' ChatID %d", newStatus, goalsSheetName, chatID); readRange := fmt.Sprintf("%s!A:F", goalsSheetName); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); if err != nil { return fmt.Errorf("читання для оновл.: %w", err) }; targetSheetRowIndex := -1; if len(resp.Values) > 0 { for i := len(resp.Values) - 1; i >= 0; i-- { row := resp.Values[i]; isHeader := (i == 0 && len(row) > 0 && (fmt.Sprintf("%v", row[0]) == "ChatID" || fmt.Sprintf("%v", row[0]) == "ID чату користувача")); if isHeader || len(row) < 6 { continue }; rowChatIDStr := fmt.Sprintf("%v", row[0]); rowStatusStr := fmt.Sprintf("%v", row[5]); rowChatID, errChatID := strconv.ParseInt(rowChatIDStr, 10, 64); if errChatID == nil && rowChatID == chatID && rowStatusStr == "Активна" { targetSheetRowIndex = i + 1; break } } }; if targetSheetRowIndex == -1 { return fmt.Errorf("не знайдено активної цілі") }; statusUpdateRange := fmt.Sprintf("%s!F%d", goalsSheetName, targetSheetRowIndex); statusValueRange := &sheets.ValueRange{Values: [][]interface{}{{newStatus}}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { return fmt.Errorf("оновлення статусу: %w", err) }; closedDateUpdateRange := fmt.Sprintf("%s!H%d", goalsSheetName, targetSheetRowIndex); closedDateValueRange := &sheets.ValueRange{Values: [][]interface{}{{closedDate.In(KyivLocation).Format("2006-01-02")}}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, closedDateUpdateRange, closedDateValueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { log.Printf("Помилка оновл. дати закриття: %v", err) }; log.Printf("Статус цілі ChatID %d (рядок %d) '%s' оновлено на '%s'", chatID, targetSheetRowIndex, goalsSheetName, newStatus); return nil
}

// AddInvestmentToSheet додає рядок з інвестицією на вказаний аркуш.
func AddInvestmentToSheet(srv *sheets.Service, spreadsheetID string, investmentsSheetName string, chatID int64, invData InvestmentData) error {
	log.Printf("Додавання інвестиції на аркуш '%s' для ChatID %d: %+v", investmentsSheetName, chatID, invData)
	var rowValues []interface{}; rowValues = append(rowValues, "", invData.Type, invData.Name, invData.AmountInvested, invData.Currency, invData.DateInvested.In(KyivLocation).Format("2006-01-02"), "", "", "Активна", "", ""); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowValues}}; appendRange := fmt.Sprintf("%s!A:K", investmentsSheetName); _, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do(); if err != nil { log.Printf("Помилка запису інвестиції '%s': %v", investmentsSheetName, err); return fmt.Errorf("не вдалося записати інвестицію: %w", err) }; log.Printf("Інвестиція для ChatID %d записана на '%s'", chatID, investmentsSheetName); return nil
}

// --- Функції для роботи з аркушем "РобочийГрафік" ---
const workLogSheetNameDefault = "РобочийГрафік"

func LogWorkStart(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, startTime time.Time) error {
	if workLogSheetName == "" { workLogSheetName = workLogSheetNameDefault }; localStartTime := startTime.In(KyivLocation); todayStr := localStartTime.Format("2006-01-02"); startTimeStr := localStartTime.Format("15:04:05"); log.Printf("Лог старту: ChatID=%d, Date=%s, Time=%s, Sheet=%s", chatID, todayStr, startTimeStr, workLogSheetName); readRange := fmt.Sprintf("%s!A:A", workLogSheetName); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); var sheetValues [][]interface{}; if err != nil { log.Printf("Помилка читання дат '%s': %v.", workLogSheetName, err) } else { sheetValues = resp.Values }; 
	rowIndexInValues, _ := findRowByDate(sheetValues, todayStr); // ВИКЛИК findRowByDate
	actualSheetRowIndex := -1; if rowIndexInValues != -1 { actualSheetRowIndex = rowIndexInValues + 1 }; 
	rowData := []interface{}{todayStr, "Розпочато", startTimeStr, "", ""}; if actualSheetRowIndex != -1 { updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, actualSheetRowIndex, actualSheetRowIndex); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { return fmt.Errorf("оновл. старту: %w", err) }; log.Printf("Рядок %d (%s) оновлено (старт).", actualSheetRowIndex, todayStr) } else { appendRange := fmt.Sprintf("%s!A:E", workLogSheetName); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}; _, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do(); if err != nil { return fmt.Errorf("додавання старту: %w", err) }; log.Printf("Новий рядок %s додано (старт).", todayStr) }; return nil
}

func LogWorkStop(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, endTime time.Time) (time.Duration, error) {
	if workLogSheetName == "" { workLogSheetName = workLogSheetNameDefault }; localEndTime := endTime.In(KyivLocation); todayStr := localEndTime.Format("2006-01-02"); endTimeStr := localEndTime.Format("15:04:05"); log.Printf("Лог стоп: ChatID=%d, Date=%s, Time=%s, Sheet=%s", chatID, todayStr, endTimeStr, workLogSheetName); zeroDuration := time.Duration(0); readRange := fmt.Sprintf("%s!A:C", workLogSheetName); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); if err != nil { return zeroDuration, fmt.Errorf("читання для стоп: %w", err) }; 
	rowIndexInValues, rowDataFromFind := findRowByDate(resp.Values, todayStr); // ВИКЛИК findRowByDate
	actualSheetRowIndex := -1; if rowIndexInValues != -1 { actualSheetRowIndex = rowIndexInValues + 1 }; 
	if actualSheetRowIndex == -1 { return zeroDuration, fmt.Errorf("не знайдено старт") }; var startTimeInKyiv time.Time; var duration time.Duration = zeroDuration; var currentStatus string = "Невідомо"; if len(rowDataFromFind) >= 3 { startTimeSheetStr := fmt.Sprintf("%v", rowDataFromFind[2]); currentStatus = fmt.Sprintf("%v", rowDataFromFind[1]); parsedStartTime, errTime := time.ParseInLocation("15:04:05", startTimeSheetStr, KyivLocation); if errTime == nil { year, month, day := localEndTime.Date(); startTimeInKyiv = time.Date(year, month, day, parsedStartTime.Hour(), parsedStartTime.Minute(), parsedStartTime.Second(), 0, KyivLocation); if (currentStatus == "Розпочато" || currentStatus == "Робочий") && localEndTime.After(startTimeInKyiv) { duration = localEndTime.Sub(startTimeInKyiv) } else { log.Printf("Статус не 'Розпочато' для %d (%s).", chatID, currentStatus) } } else { log.Printf("Не розпарсено час '%s': %v", startTimeSheetStr, errTime) } } else { log.Printf("Мало даних у рядку %d.", actualSheetRowIndex) }; durationStr := FormatDuration(duration); newStatus := "Завершено"; updateRangeDE := fmt.Sprintf("%s!D%d:E%d", workLogSheetName, actualSheetRowIndex, actualSheetRowIndex); valueRangeDE := &sheets.ValueRange{Values: [][]interface{}{{endTimeStr, durationStr}}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRangeDE, valueRangeDE).ValueInputOption("USER_ENTERED").Do(); if err != nil { log.Printf("Помилка оновл. D:E %d: %v", actualSheetRowIndex, err) }; statusUpdateRange := fmt.Sprintf("%s!B%d", workLogSheetName, actualSheetRowIndex); statusValueRange := &sheets.ValueRange{Values: [][]interface{}{{newStatus}}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { log.Printf("Помилка оновл. статусу %d: %v", actualSheetRowIndex, err); return zeroDuration, fmt.Errorf("оновл. стоп запису: %w", err) }; log.Printf("Рядок %d (%s) оновлено (завершено, %s).", actualSheetRowIndex, todayStr, durationStr); return duration, nil
}

func LogDayOff(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, dateToLog time.Time) error {
	if workLogSheetName == "" { workLogSheetName = workLogSheetNameDefault }; localDate := dateToLog.In(KyivLocation); dateStr := localDate.Format("2006-01-02"); log.Printf("Лог вихідного: ChatID=%d, Date=%s, Sheet=%s", chatID, dateStr, workLogSheetName); readRange := fmt.Sprintf("%s!A:A", workLogSheetName); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); var sheetValues [][]interface{}; if err != nil { log.Printf("Помилка читання дат '%s': %v.", workLogSheetName, err) } else { sheetValues = resp.Values }; 
	rowIndexInValues, _ := findRowByDate(sheetValues, dateStr); // ВИКЛИК findRowByDate
	actualSheetRowIndex := -1; if rowIndexInValues != -1 { actualSheetRowIndex = rowIndexInValues + 1 }; 
	rowData := []interface{}{dateStr, "Вихідний", "", "", ""}; if actualSheetRowIndex != -1 { updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, actualSheetRowIndex, actualSheetRowIndex); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { return fmt.Errorf("оновл. вихідного: %w", err) }; log.Printf("Рядок %d (%s) оновлено (вихідний).", actualSheetRowIndex, dateStr) } else { appendRange := fmt.Sprintf("%s!A:E", workLogSheetName); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}; _, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do(); if err != nil { return fmt.Errorf("додавання вихідного: %w", err) }; log.Printf("Новий рядок %s додано (вихідний).", dateStr) }; return nil
}

// GetActiveGoalFromSheet завантажує останню активну ціль
func GetActiveGoalFromSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64) (FinancialGoalData, bool, error) {
    var goalData FinancialGoalData; var found bool; readRange := fmt.Sprintf("%s!A:G", goalsSheetName); log.Printf("Пошук цілі для ChatID %d на '%s'", chatID, goalsSheetName); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); if err != nil { return goalData, false, fmt.Errorf("не прочитано цілі: %w", err) }; if len(resp.Values) > 1 { for i := len(resp.Values) - 1; i >= 0; i-- { row := resp.Values[i]; isHeader := (i == 0 && len(row) > 0 && (fmt.Sprintf("%v", row[0]) == "ChatID" || fmt.Sprintf("%v", row[0]) == "ID чату користувача")); if isHeader || len(row) < 7 { continue }; rowChatIDStr := fmt.Sprintf("%v", row[0]); rowStatusStr := fmt.Sprintf("%v", row[5]); rowChatID, errChatID := strconv.ParseInt(rowChatIDStr, 10, 64); if errChatID == nil && rowChatID == chatID && rowStatusStr == "Активна" { goalData.Amount, _ = strconv.ParseFloat(fmt.Sprintf("%v", row[1]), 64); goalData.Currency = fmt.Sprintf("%v", row[2]); goalData.Days, _ = strconv.Atoi(fmt.Sprintf("%v", row[3])); setDateStr := fmt.Sprintf("%v", row[4]); goalData.OriginalText = fmt.Sprintf("%v", row[6]); parsedSetDate, errDate := time.ParseInLocation("2006-01-02", setDateStr, KyivLocation); if errDate == nil { goalData.SetDate = parsedSetDate.UTC() } else { goalData.SetDate = time.Now().UTC() }; found = true; log.Printf("Знайдено ціль для %d '%s': %+v", chatID, goalsSheetName, goalData); break } } }; if !found { log.Printf("Активну ціль для %d на '%s' не знайдено.", chatID, goalsSheetName) }; return goalData, found, nil
}

// CountWorkingDaysInRange підраховує кількість робочих днів
func CountWorkingDaysInRange(srv *sheets.Service, spreadsheetID string, workLogSheetName string, startDate, endDate time.Time) (int, error) {
    if workLogSheetName == "" { workLogSheetName = workLogSheetNameDefault }; readRange := fmt.Sprintf("%s!A:B", workLogSheetName); log.Printf("Підрахунок роб. днів: '%s' для %s - %s", readRange, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); if err != nil { return 0, fmt.Errorf("не прочитано графік: %w", err) }; dayStatusMap := make(map[string]string); if len(resp.Values) > 0 { for i, row := range resp.Values { isHeader := (i == 0 && len(row) > 0 && (fmt.Sprintf("%v", row[0]) == "Дата" || fmt.Sprintf("%v", row[0]) == "Date")); if isHeader { continue }; if len(row) >= 2 { dayStatusMap[fmt.Sprintf("%v", row[0])] = fmt.Sprintf("%v", row[1]) } } }; log.Printf("Мапа статусів з '%s': %v", workLogSheetName, dayStatusMap); workingDays := 0; currentDay := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, KyivLocation); lastDay := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, KyivLocation); for !currentDay.After(lastDay) { dateStr := currentDay.Format("2006-01-02"); status, exists := dayStatusMap[dateStr]; isWorkingDay := true; if exists && status == "Вихідний" { isWorkingDay = false }; if isWorkingDay { workingDays++; /* log.Printf("День %s роб. (статус: '%s', є: %t)", dateStr, status, exists) */ } else { log.Printf("День %s ВИХ. (статус: '%s')", dateStr, status) }; currentDay = currentDay.AddDate(0, 0, 1) }; log.Printf("Знайдено %d роб. днів у діапазоні %s - %s", workingDays, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")); return workingDays, nil
}

// Функція getMotivation видалена звідси
