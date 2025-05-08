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
func init() {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil { log.Printf("Критична помилка: не вдалося завантажити часову зону Europe/Kyiv: %v.", err); KyivLocation = time.UTC } else { KyivLocation = loc; log.Println("Часову зону Europe/Kyiv успішно завантажено (пакет sheets).") }
}
func GetCurrentTimeInKyiv() time.Time { return time.Now().In(KyivLocation) }

// ---- Допоміжні функції ----
func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) (int, []interface{}) { // ... (код без змін) ...
	if len(sheetData) == 0 { return -1, nil }; startRowIndex := 0
	if len(sheetData) > 0 && len(sheetData[0]) > 0 && (fmt.Sprintf("%v", sheetData[0][0]) == "Дата" || fmt.Sprintf("%v", sheetData[0][0]) == "Date" || fmt.Sprintf("%v", sheetData[0][0]) == "ChatID" || fmt.Sprintf("%v", sheetData[0][0]) == "ID чату користувача") { startRowIndex = 1 }
	for i := startRowIndex; i < len(sheetData); i++ { row := sheetData[i]; if len(row) > 0 { if fmt.Sprintf("%v", row[0]) == dateToFind { return i, row } } }; return -1, nil
}
func FormatDuration(d time.Duration) string { // ... (код без змін) ...
	d = d.Round(time.Minute); h := d / time.Hour; d -= h * time.Hour; m := d / time.Minute; return fmt.Sprintf("%dh %dm", h, m)
}

// --- Функції роботи з Google Sheets API ---
func NewService(credentialsJSON []byte) (*sheets.Service, error) { /*...*/ return nil, nil } // Залишаємо заглушку

// GenerateProgressReport тепер повертає помилку, якщо числові дані некоректні
func GenerateProgressReport(srv *sheets.Service, spreadsheetID string, reportSheetRange string) (SheetRowData, error) {
	var data SheetRowData
	var err error // Для помилок парсингу

	log.Printf("Спроба читання даних з Google Sheets: SpreadsheetID=%s, Range=%s", spreadsheetID, reportSheetRange)
	resp, errGet := srv.Spreadsheets.Values.Get(spreadsheetID, reportSheetRange).Do()
	if errGet != nil {
		log.Printf("Не вдалося отримати дані з Google Sheets (GenerateProgressReport): %v", errGet)
		return data, fmt.Errorf("помилка отримання даних з Google Sheets: %w", errGet)
	}

	if len(resp.Values) < 1 || len(resp.Values[0]) < 5 {
		errMsg := fmt.Sprintf("недостатньо даних у діапазоні %s (очікується 1 рядок з 5 колонками)", reportSheetRange)
		log.Println(errMsg)
		return data, fmt.Errorf(errMsg)
	}

	row := resp.Values[0]

	// Парсинг з поверненням помилки у разі невдачі
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) } else { data.Date = "" }

	if len(row) > 1 {
		incomeStr := fmt.Sprintf("%v", row[1])
		data.Income, err = strconv.ParseFloat(incomeStr, 64)
		if err != nil {
			log.Printf("Помилка парсингу доходу '%s': %v.", incomeStr, err)
			return data, fmt.Errorf("некоректне значення Доходу в таблиці ('%s')", incomeStr) // Повертаємо помилку
		}
	} else { return data, fmt.Errorf("відсутнє значення Доходу в таблиці (колонка B)") }

	if len(row) > 2 {
		sheetGoalStr := fmt.Sprintf("%v", row[2])
		data.SheetGoal, err = strconv.ParseFloat(sheetGoalStr, 64)
		if err != nil {
			log.Printf("Помилка парсингу мети з таблиці '%s': %v.", sheetGoalStr, err)
			return data, fmt.Errorf("некоректне значення Мети в таблиці ('%s')", sheetGoalStr) // Повертаємо помилку
		}
	} else { return data, fmt.Errorf("відсутнє значення Мети в таблиці (колонка C)") }

	if len(row) > 3 {
		sheetDaysLeftStr := fmt.Sprintf("%v", row[3])
		data.SheetDaysLeft, err = strconv.Atoi(sheetDaysLeftStr)
		if err != nil {
			log.Printf("Помилка парсингу 'днів залишилося' з таблиці '%s': %v.", sheetDaysLeftStr, err)
			return data, fmt.Errorf("некоректний формат 'Днів залишилося' в таблиці ('%s')", sheetDaysLeftStr) // Повертаємо помилку
		}
	} else { return data, fmt.Errorf("відсутнє значення 'Днів залишилося' в таблиці (колонка D)") }

	if len(row) > 4 {
		sheetReqDailyStr := fmt.Sprintf("%v", row[4])
		data.SheetReqDaily, err = strconv.ParseFloat(sheetReqDailyStr, 64)
		if err != nil {
			log.Printf("Помилка парсингу 'потрібно щодня' з таблиці '%s': %v.", sheetReqDailyStr, err)
			return data, fmt.Errorf("некоректний формат 'Потрібно щодня' в таблиці ('%s')", sheetReqDailyStr) // Повертаємо помилку
		}
	} else { return data, fmt.Errorf("відсутнє значення 'Потрібно щодня' в таблиці (колонка E)") }

	log.Printf("Дані з аркуша '%s' успішно розпарсені: %+v", reportSheetRange, data)
	return data, nil // Повертаємо дані та nil помилку
}

// AddGoalToSheet ... (код без змін) ...
func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, goalData FinancialGoalData) error { /* ... */ 
	log.Printf("Додавання цілі на аркуш '%s' для ChatID %d: %+v", goalsSheetName, chatID, goalData); var rowValues []interface{}; rowValues = append(rowValues, chatID, goalData.Amount, goalData.Currency, goalData.Days, goalData.SetDate.In(KyivLocation).Format("2006-01-02"), "Активна", goalData.OriginalText, ""); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowValues}}; appendRange := fmt.Sprintf("%s!A:H", goalsSheetName); _, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do(); if err != nil { log.Printf("Помилка запису цілі на аркуш '%s': %v", goalsSheetName, err); return fmt.Errorf("не вдалося записати ціль: %w", err) }; log.Printf("Ціль для ChatID %d успішно записана на аркуш '%s'", chatID, goalsSheetName); return nil
}
// UpdateGoalStatusInSheet ... (код без змін) ...
func UpdateGoalStatusInSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, newStatus string, closedDate time.Time) error { /* ... */
	log.Printf("Оновлення статусу цілі на '%s' на аркуші '%s' для ChatID %d", newStatus, goalsSheetName, chatID); readRange := fmt.Sprintf("%s!A:F", goalsSheetName); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); if err != nil { log.Printf("Помилка читання '%s': %v", readRange, err); return fmt.Errorf("не вдалося прочитати дані: %w", err) }; targetSheetRowIndex := -1; if len(resp.Values) > 1 { for i := len(resp.Values) - 1; i >= 0; i-- { row := resp.Values[i]; if i == 0 && (len(row) > 0 && (fmt.Sprintf("%v", row[0]) == "ChatID" || fmt.Sprintf("%v", row[0]) == "ID чату користувача")) { continue }; if len(row) < 6 { continue }; rowChatIDStr := fmt.Sprintf("%v", row[0]); rowStatusStr := fmt.Sprintf("%v", row[5]); rowChatID, errChatID := strconv.ParseInt(rowChatIDStr, 10, 64); if errChatID == nil && rowChatID == chatID && rowStatusStr == "Активна" { targetSheetRowIndex = i + 1; break } } }; if targetSheetRowIndex == -1 { log.Printf("Не знайдено активної цілі для ChatID %d на '%s'", chatID, goalsSheetName); return fmt.Errorf("не знайдено активної цілі для оновлення") }; statusUpdateRange := fmt.Sprintf("%s!F%d", goalsSheetName, targetSheetRowIndex); statusValueRange := &sheets.ValueRange{Values: [][]interface{}{{newStatus}}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { log.Printf("Помилка оновлення статусу цілі: %v", err); return fmt.Errorf("не вдалося оновити статус цілі: %w", err) }; closedDateUpdateRange := fmt.Sprintf("%s!H%d", goalsSheetName, targetSheetRowIndex); closedDateValueRange := &sheets.ValueRange{Values: [][]interface{}{{closedDate.In(KyivLocation).Format("2006-01-02")}}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, closedDateUpdateRange, closedDateValueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { log.Printf("Помилка оновлення дати закриття цілі: %v", err) }; log.Printf("Статус цілі для ChatID %d (рядок %d) '%s' оновлено на '%s'", chatID, targetSheetRowIndex, goalsSheetName, newStatus); return nil
}
// --- Функції для роботи з аркушем "РобочийГрафік" ---
const workLogSheetNameDefault = "РобочийГрафік"
// LogWorkStart ... (код без змін) ...
func LogWorkStart(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, startTime time.Time) error { /* ... */
	if workLogSheetName == "" { workLogSheetName = workLogSheetNameDefault }; localStartTime := startTime.In(KyivLocation); todayStr := localStartTime.Format("2006-01-02"); startTimeStr := localStartTime.Format("15:04:05"); log.Printf("Логування початку роботи для ChatID %d на %s, час: %s (Аркуш: %s)", chatID, todayStr, startTimeStr, workLogSheetName); readRange := fmt.Sprintf("%s!A:A", workLogSheetName); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); var sheetValues [][]interface{}; if err != nil { log.Printf("Помилка читання дат з '%s': %v.", workLogSheetName, err) } else { sheetValues = resp.Values }; rowIndexInValues, _ := findRowByDate(sheetValues, todayStr); actualSheetRowIndex := -1; if rowIndexInValues != -1 { actualSheetRowIndex = rowIndexInValues + 1 }; rowData := []interface{}{todayStr, "Розпочато", startTimeStr, "", ""}; if actualSheetRowIndex != -1 { updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, actualSheetRowIndex, actualSheetRowIndex); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { return fmt.Errorf("не вдалося оновити запис: %w", err) }; log.Printf("Рядок %d для дати %s оновлено.", actualSheetRowIndex, todayStr) } else { appendRange := fmt.Sprintf("%s!A:E", workLogSheetName); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}; _, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do(); if err != nil { return fmt.Errorf("не вдалося додати запис: %w", err) }; log.Printf("Новий рядок для дати %s додано.", todayStr) }; return nil
}
// LogWorkStop ... (код без змін) ...
func LogWorkStop(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, endTime time.Time) (time.Duration, error) { /* ... */ 
	if workLogSheetName == "" { workLogSheetName = workLogSheetNameDefault }; localEndTime := endTime.In(KyivLocation); todayStr := localEndTime.Format("2006-01-02"); endTimeStr := localEndTime.Format("15:04:05"); log.Printf("Логування завершення для ChatID %d на %s, час: %s (%s)", chatID, todayStr, endTimeStr, workLogSheetName); zeroDuration := time.Duration(0); readRange := fmt.Sprintf("%s!A:C", workLogSheetName); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); if err != nil { return zeroDuration, fmt.Errorf("не вдалося прочитати дані: %w", err) }; rowIndexInValues, rowDataFromFind := findRowByDate(resp.Values, todayStr); actualSheetRowIndex := -1; if rowIndexInValues != -1 { actualSheetRowIndex = rowIndexInValues + 1 }; if actualSheetRowIndex == -1 { return zeroDuration, fmt.Errorf("не знайдено запису про початок") }; var startTimeInKyiv time.Time; var duration time.Duration = zeroDuration; var currentStatus string = "Невідомо"; if len(rowDataFromFind) >= 3 { startTimeSheetStr := fmt.Sprintf("%v", rowDataFromFind[2]); currentStatus = fmt.Sprintf("%v", rowDataFromFind[1]); parsedStartTime, errTime := time.ParseInLocation("15:04:05", startTimeSheetStr, KyivLocation); if errTime == nil { year, month, day := localEndTime.Date(); startTimeInKyiv = time.Date(year, month, day, parsedStartTime.Hour(), parsedStartTime.Minute(), parsedStartTime.Second(), 0, KyivLocation); if (currentStatus == "Розпочато" || currentStatus == "Робочий") && localEndTime.After(startTimeInKyiv) { duration = localEndTime.Sub(startTimeInKyiv) } else { log.Printf("Статус не 'Розпочато' або час некоректний для ChatID %d (%s).", chatID, currentStatus) } } else { log.Printf("Не вдалося парсити час початку '%s': %v", startTimeSheetStr, errTime) } } else { log.Printf("Недостатньо даних у рядку %d.", actualSheetRowIndex) }; durationStr := FormatDuration(duration); newStatus := "Завершено"; updateRangeDE := fmt.Sprintf("%s!D%d:E%d", workLogSheetName, actualSheetRowIndex, actualSheetRowIndex); valueRangeDE := &sheets.ValueRange{Values: [][]interface{}{{endTimeStr, durationStr}}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRangeDE, valueRangeDE).ValueInputOption("USER_ENTERED").Do(); if err != nil { log.Printf("Помилка оновлення D:E рядка %d: %v", actualSheetRowIndex, err) }; statusUpdateRange := fmt.Sprintf("%s!B%d", workLogSheetName, actualSheetRowIndex); statusValueRange := &sheets.ValueRange{Values: [][]interface{}{{newStatus}}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { log.Printf("Помилка оновлення статусу в рядку %d: %v", actualSheetRowIndex, err); return zeroDuration, fmt.Errorf("не вдалося оновити запис: %w", err) }; log.Printf("Рядок %d для %s оновлено (завершено, %s).", actualSheetRowIndex, todayStr, durationStr); return duration, nil
}
// LogDayOff ... (код без змін) ...
func LogDayOff(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, dateToLog time.Time) error { /* ... */
	if workLogSheetName == "" { workLogSheetName = workLogSheetNameDefault }; localDate := dateToLog.In(KyivLocation); dateStr := localDate.Format("2006-01-02"); log.Printf("Логування вихідного для ChatID %d на %s (%s)", chatID, dateStr, workLogSheetName); readRange := fmt.Sprintf("%s!A:A", workLogSheetName); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); var sheetValues [][]interface{}; if err != nil { log.Printf("Помилка читання дат з '%s': %v.", workLogSheetName, err) } else { sheetValues = resp.Values }; rowIndexInValues, _ := findRowByDate(sheetValues, dateStr); actualSheetRowIndex := -1; if rowIndexInValues != -1 { actualSheetRowIndex = rowIndexInValues + 1 }; rowData := []interface{}{dateStr, "Вихідний", "", "", ""}; if actualSheetRowIndex != -1 { updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, actualSheetRowIndex, actualSheetRowIndex); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}; _, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).ValueInputOption("USER_ENTERED").Do(); if err != nil { return fmt.Errorf("не вдалося оновити запис: %w", err) }; log.Printf("Рядок %d для %s оновлено (статус 'Вихідний').", actualSheetRowIndex, dateStr) } else { appendRange := fmt.Sprintf("%s!A:E", workLogSheetName); valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}; _, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do(); if err != nil { return fmt.Errorf("не вдалося додати запис: %w", err) }; log.Printf("Новий рядок для %s додано (статус 'Вихідний').", dateStr) }; return nil
}
// GetActiveGoalFromSheet ... (код без змін) ...
func GetActiveGoalFromSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64) (FinancialGoalData, bool, error) { /* ... */
	var goalData FinancialGoalData; var found bool; readRange := fmt.Sprintf("%s!A:G", goalsSheetName); log.Printf("Пошук активної цілі для ChatID %d на '%s'", chatID, goalsSheetName); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); if err != nil { return goalData, false, fmt.Errorf("не вдалося прочитати дані: %w", err) }; if len(resp.Values) > 1 { for i := len(resp.Values) - 1; i >= 0; i-- { row := resp.Values[i]; if i == 0 && (len(row) > 0 && (fmt.Sprintf("%v", row[0]) == "ChatID" || fmt.Sprintf("%v", row[0]) == "ID чату користувача")) { continue }; if len(row) < 7 { continue }; rowChatIDStr := fmt.Sprintf("%v", row[0]); rowStatusStr := fmt.Sprintf("%v", row[5]); rowChatID, errChatID := strconv.ParseInt(rowChatIDStr, 10, 64); if errChatID == nil && rowChatID == chatID && rowStatusStr == "Активна" { goalData.Amount, _ = strconv.ParseFloat(fmt.Sprintf("%v", row[1]), 64); goalData.Currency = fmt.Sprintf("%v", row[2]); goalData.Days, _ = strconv.Atoi(fmt.Sprintf("%v", row[3])); setDateStr := fmt.Sprintf("%v", row[4]); goalData.OriginalText = fmt.Sprintf("%v", row[6]); parsedSetDate, errDate := time.ParseInLocation("2006-01-02", setDateStr, KyivLocation); if errDate == nil { goalData.SetDate = parsedSetDate.UTC() } else { goalData.SetDate = time.Now().UTC() }; found = true; log.Printf("Знайдено активну ціль для ChatID %d '%s': %+v", chatID, goalsSheetName, goalData); break } } }; if !found { log.Printf("Активну ціль для ChatID %d на '%s' не знайдено.", chatID, goalsSheetName) }; return goalData, found, nil
}
// CountWorkingDaysInRange ... (код без змін) ...
func CountWorkingDaysInRange(srv *sheets.Service, spreadsheetID string, workLogSheetName string, startDate, endDate time.Time) (int, error) { /* ... */
	if workLogSheetName == "" { workLogSheetName = workLogSheetNameDefault }; readRange := fmt.Sprintf("%s!A:B", workLogSheetName); log.Printf("Підрахунок робочих днів: '%s' для %s - %s", readRange, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")); resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do(); if err != nil { return 0, fmt.Errorf("не вдалося прочитати графік: %w", err) }; dayStatusMap := make(map[string]string); if len(resp.Values) > 0 { for i, row := range resp.Values { if i == 0 && (len(row) > 0 && (fmt.Sprintf("%v", row[0]) == "Дата" || fmt.Sprintf("%v", row[0]) == "Date")) { continue }; if len(row) >= 2 { dayStatusMap[fmt.Sprintf("%v", row[0])] = fmt.Sprintf("%v", row[1]) } } }; log.Printf("Мапа статусів з '%s': %v", workLogSheetName, dayStatusMap); workingDays := 0; currentDay := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, KyivLocation); lastDay := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, KyivLocation); for !currentDay.After(lastDay) { dateStr := currentDay.Format("2006-01-02"); status, exists := dayStatusMap[dateStr]; isWorkingDay := true; if exists && status == "Вихідний" { isWorkingDay = false }; if isWorkingDay { workingDays++; log.Printf("День %s роб. (статус: '%s', є: %t)", dateStr, status, exists) } else { log.Printf("День %s ВИХ. (статус: '%s')", dateStr, status) }; currentDay = currentDay.AddDate(0, 0, 1) }; log.Printf("Знайдено %d роб. днів у діапазоні %s - %s", workingDays, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")); return workingDays, nil
}
