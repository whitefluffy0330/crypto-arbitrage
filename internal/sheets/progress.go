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

// Дозвіл на читання та запис
const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets"

// ---- Структури ----
type SheetRowData struct { // Дані з аркуша "Звіт"
	Date          string
	Income        float64
	SheetGoal     float64
	SheetDaysLeft int
	SheetReqDaily float64
}

type FinancialGoalData struct { // Дані для запису/читання цілі
	Amount       float64
	Currency     string
	Days         int
	OriginalText string
	SetDate      time.Time
}

// ---- Часова зона ----
var kyivLocation *time.Location
func init() {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Printf("Критична помилка: не вдалося завантажити часову зону Europe/Kyiv: %v. Буде використано UTC.", err)
		kyivLocation = time.UTC
	} else {
		kyivLocation = loc
		log.Println("Часову зону Europe/Kyiv успішно завантажено (пакет sheets).")
	}
}
func getCurrentTimeInKyiv() time.Time { return time.Now().In(kyivLocation) }

// ---- Допоміжні функції ----
func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) (int, []interface{}) {
	if len(sheetData) == 0 { return -1, nil }
	startRowIndex := 0
	if len(sheetData) > 0 && len(sheetData[0]) > 0 && (fmt.Sprintf("%v", sheetData[0][0]) == "Дата" || fmt.Sprintf("%v", sheetData[0][0]) == "Date") { startRowIndex = 1 }
	for i := startRowIndex; i < len(sheetData); i++ {
		row := sheetData[i]
		if len(row) > 0 {
			if fmt.Sprintf("%v", row[0]) == dateToFind { return i, row }
		}
	}
	return -1, nil
}
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Minute); h := d / time.Hour; d -= h * time.Hour; m := d / time.Minute; return fmt.Sprintf("%dh %dm", h, m)
}

// --- Функції роботи з Google Sheets API ---

// NewService - не використовується
func NewService(credentialsJSON []byte) (*sheets.Service, error) { /* ... */ 
	ctx := context.Background()
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(credentialsJSON))
	if err != nil {
		return nil, fmt.Errorf("не вдалося створити клієнт Sheets: %w", err)
	}
	return srv, nil
}

// GenerateProgressReport тепер приймає reportSheetRange (напр., "Звіт!A2:E2")
func GenerateProgressReport(srv *sheets.Service, spreadsheetID string, reportSheetRange string) (SheetRowData, error) {
	var data SheetRowData
	var err error // Оголошуємо err тут для використання в парсингу

	log.Printf("Спроба читання даних з Google Sheets: SpreadsheetID=%s, Range=%s", spreadsheetID, reportSheetRange)
	resp, errGet := srv.Spreadsheets.Values.Get(spreadsheetID, reportSheetRange).Do()
	if errGet != nil {
		log.Printf("Не вдалося отримати дані з Google Sheets (GenerateProgressReport): %v", errGet)
		return data, fmt.Errorf("помилка отримання даних з Google Sheets: %w", errGet)
	}

	if len(resp.Values) < 1 || len(resp.Values[0]) < 5 {
		errMsg := fmt.Sprintf("недостатньо даних у діапазоні %s", reportSheetRange)
		log.Println(errMsg)
		return data, fmt.Errorf(errMsg)
	}

	row := resp.Values[0]
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) }
	if len(row) > 1 { incomeStr := fmt.Sprintf("%v", row[1]); data.Income, err = strconv.ParseFloat(incomeStr, 64); if err != nil { log.Printf("Помилка парсингу доходу '%s': %v. Встановлено 0.", incomeStr, err); data.Income = 0 }}
	if len(row) > 2 { sheetGoalStr := fmt.Sprintf("%v", row[2]); data.SheetGoal, err = strconv.ParseFloat(sheetGoalStr, 64); if err != nil { log.Printf("Помилка парсингу мети з таблиці '%s': %v. Встановлено 0.", sheetGoalStr, err); data.SheetGoal = 0 }}
	if len(row) > 3 { sheetDaysLeftStr := fmt.Sprintf("%v", row[3]); data.SheetDaysLeft, err = strconv.Atoi(sheetDaysLeftStr); if err != nil { log.Printf("Помилка парсингу 'днів залишилося' з таблиці '%s': %v. Встановлено 0.", sheetDaysLeftStr, err); data.SheetDaysLeft = 0 }}
	if len(row) > 4 { sheetReqDailyStr := fmt.Sprintf("%v", row[4]); data.SheetReqDaily, err = strconv.ParseFloat(sheetReqDailyStr, 64); if err != nil { log.Printf("Помилка парсингу 'потрібно щодня' з таблиці '%s': %v. Встановлено 0.", sheetReqDailyStr, err); data.SheetReqDaily = 0 }}
	
	log.Printf("Дані з аркуша '%s' успішно розпарсені: %+v", reportSheetRange, data)
	return data, nil
}

// AddGoalToSheet тепер приймає goalsSheetName (напр., "МоїЦілі")
func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, goalData FinancialGoalData) error {
	log.Printf("Додавання цілі на аркуш '%s' для ChatID %d: %+v", goalsSheetName, chatID, goalData)

	var rowValues []interface{}
	rowValues = append(rowValues, chatID, goalData.Amount, goalData.Currency, goalData.Days,
		goalData.SetDate.In(kyivLocation).Format("2006-01-02"), "Активна", goalData.OriginalText, "")

	valueRange := &sheets.ValueRange{Values: [][]interface{}{rowValues}}
	appendRange := fmt.Sprintf("%s!A:H", goalsSheetName) // Припускаємо, що пишемо завжди в колонки A-H

	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
		ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()

	if err != nil {
		log.Printf("Помилка запису цілі на аркуш '%s' для ChatID %d: %v", goalsSheetName, chatID, err)
		return fmt.Errorf("не вдалося записати ціль у таблицю: %w", err)
	}
	log.Printf("Ціль для ChatID %d успішно записана на аркуш '%s'", chatID, goalsSheetName)
	return nil
}

// UpdateGoalStatusInSheet тепер приймає goalsSheetName
func UpdateGoalStatusInSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64, newStatus string, closedDate time.Time) error {
	log.Printf("Оновлення статусу цілі на '%s' на аркуші '%s' для ChatID %d", newStatus, goalsSheetName, chatID)

	readRange := fmt.Sprintf("%s!A:F", goalsSheetName) // Читаємо до колонки статусу F
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання даних з аркуша '%s' для оновлення статусу (ChatID %d): %v", goalsSheetName, chatID, err)
		return fmt.Errorf("не вдалося прочитати дані для оновлення статусу: %w", err)
	}

	targetSheetRowIndex := -1
	if len(resp.Values) > 1 {
		for i := len(resp.Values) - 1; i >= 1; i-- {
			row := resp.Values[i]
			if len(row) < 6 { continue }
			rowChatIDStr := fmt.Sprintf("%v", row[0])
			rowStatusStr := fmt.Sprintf("%v", row[5])
			rowChatID, errChatID := strconv.ParseInt(rowChatIDStr, 10, 64)
			if errChatID == nil && rowChatID == chatID && rowStatusStr == "Активна" {
				targetSheetRowIndex = i + 1
				break
			}
		}
	}

	if targetSheetRowIndex == -1 {
		log.Printf("Не знайдено активної цілі для ChatID %d на аркуші '%s' для оновлення статусу.", chatID, goalsSheetName)
		return fmt.Errorf("не знайдено активної цілі для оновлення")
	}

	statusUpdateRange := fmt.Sprintf("%s!F%d", goalsSheetName, targetSheetRowIndex)
	statusValueRange := &sheets.ValueRange{Values: [][]interface{}{{newStatus}}}
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення статусу цілі: %v", err)
		return fmt.Errorf("не вдалося оновити статус цілі: %w", err)
	}

	closedDateUpdateRange := fmt.Sprintf("%s!H%d", goalsSheetName, targetSheetRowIndex)
	closedDateValueRange := &sheets.ValueRange{Values: [][]interface{}{{closedDate.In(kyivLocation).Format("2006-01-02")}}}
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, closedDateUpdateRange, closedDateValueRange).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення дати закриття цілі: %v", err)
		// Не повертаємо помилку тут, якщо статус вже оновлено? Вирішуйте залежно від бізнес-логіки.
	}

	log.Printf("Статус цілі для ChatID %d (рядок %d) на аркуші '%s' успішно оновлено на '%s'", chatID, targetSheetRowIndex, goalsSheetName, newStatus)
	return nil
}

// GetActiveGoalFromSheet тепер приймає goalsSheetName
func GetActiveGoalFromSheet(srv *sheets.Service, spreadsheetID string, goalsSheetName string, chatID int64) (FinancialGoalData, bool, error) {
	var goalData FinancialGoalData
	var found bool

	readRange := fmt.Sprintf("%s!A:G", goalsSheetName) // Читаємо до OriginalText
	log.Printf("Пошук активної цілі для ChatID %d на аркуші '%s'", chatID, goalsSheetName)

	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання даних з аркуша '%s' для пошуку активної цілі (ChatID %d): %v", goalsSheetName, chatID, err)
		return goalData, false, fmt.Errorf("не вдалося прочитати дані цілей: %w", err)
	}

	if len(resp.Values) > 1 {
		for i := len(resp.Values) - 1; i >= 1; i-- {
			row := resp.Values[i]
			if len(row) >= 7 {
				rowChatIDStr := fmt.Sprintf("%v", row[0])
				rowStatusStr := fmt.Sprintf("%v", row[5])
				rowChatID, errChatID := strconv.ParseInt(rowChatIDStr, 10, 64)

				if errChatID == nil && rowChatID == chatID && rowStatusStr == "Активна" {
					amountStr := fmt.Sprintf("%v", row[1])
					currencyStr := fmt.Sprintf("%v", row[2])
					daysStr := fmt.Sprintf("%v", row[3])
					setDateStr := fmt.Sprintf("%v", row[4])
					originalTextStr := fmt.Sprintf("%v", row[6])

					goalData.Amount, _ = strconv.ParseFloat(amountStr, 64)
					goalData.Currency = currencyStr
					goalData.Days, _ = strconv.Atoi(daysStr)
					goalData.OriginalText = originalTextStr
					parsedSetDate, errDate := time.ParseInLocation("2006-01-02", setDateStr, kyivLocation)
					if errDate == nil { goalData.SetDate = parsedSetDate.UTC() } else { goalData.SetDate = time.Now().UTC() }
					found = true
					log.Printf("Знайдено активну ціль для ChatID %d на аркуші '%s': %+v", chatID, goalsSheetName, goalData)
					break
				}
			}
		}
	}
	if !found { log.Printf("Активну ціль для ChatID %d на аркуші '%s' не знайдено.", chatID, goalsSheetName) }
	return goalData, found, nil
}

// --- Функції для роботи з аркушем "РобочийГрафік" ---
// Тепер приймають workLogSheetName

func LogWorkStart(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, startTime time.Time) error {
	localStartTime := startTime.In(kyivLocation)
	todayStr := localStartTime.Format("2006-01-02")
	startTimeStr := localStartTime.Format("15:04:05")
	log.Printf("Логування початку роботи для ChatID %d на %s, час: %s (Аркуш: %s)", chatID, todayStr, startTimeStr, workLogSheetName)

	readRange := fmt.Sprintf("%s!A:A", workLogSheetName)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	var sheetValues [][]interface{}
	if err != nil { log.Printf("Помилка читання дат з '%s': %v. Спробуємо додати новий рядок.", workLogSheetName, err) } else { sheetValues = resp.Values }
	
	rowIndexInValues, _ := findRowByDate(sheetValues, todayStr)
	actualSheetRowIndex := -1
	if rowIndexInValues != -1 { actualSheetRowIndex = rowIndexInValues + 1 }

	rowData := []interface{}{todayStr, "Розпочато", startTimeStr, "", ""}
	if actualSheetRowIndex != -1 {
		log.Printf("Знайдено рядок %d для дати %s. Оновлення...", actualSheetRowIndex, todayStr)
		updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, actualSheetRowIndex, actualSheetRowIndex)
		valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}
		_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).ValueInputOption("USER_ENTERED").Do()
		if err != nil { return fmt.Errorf("не вдалося оновити запис про початок роботи: %w", err) }
		log.Printf("Рядок %d для дати %s успішно оновлено (роботу розпочато).", actualSheetRowIndex, todayStr)
	} else {
		log.Printf("Не знайдено рядка для дати %s. Додавання нового...", todayStr)
		appendRange := fmt.Sprintf("%s!A:E", workLogSheetName) // Використовуємо конкретний діапазон для Append
		valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}
		_, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()
		if err != nil { return fmt.Errorf("не вдалося додати запис про початок роботи: %w", err) }
		log.Printf("Новий рядок для дати %s успішно додано (роботу розпочато).", todayStr)
	}
	return nil
}

func LogWorkStop(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, endTime time.Time) (time.Duration, error) {
	localEndTime := endTime.In(kyivLocation)
	todayStr := localEndTime.Format("2006-01-02")
	endTimeStr := localEndTime.Format("15:04:05")
	log.Printf("Логування завершення роботи для ChatID %d на %s, час: %s (Аркуш: %s)", chatID, todayStr, endTimeStr, workLogSheetName)
	zeroDuration := time.Duration(0)

	readRange := fmt.Sprintf("%s!A:C", workLogSheetName)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil { return zeroDuration, fmt.Errorf("не вдалося прочитати дані для завершення роботи: %w", err) }
	
	rowIndexInValues, rowDataFromFind := findRowByDate(resp.Values, todayStr)
	actualSheetRowIndex := -1
	if rowIndexInValues != -1 { actualSheetRowIndex = rowIndexInValues + 1 }

	if actualSheetRowIndex == -1 { return zeroDuration, fmt.Errorf("не знайдено запису про початок роботи за сьогодні") }

	var startTimeInKyiv time.Time
	var duration time.Duration = zeroDuration
	var currentStatus string = "Невідомо"
	if len(rowDataFromFind) >= 3 {
		startTimeSheetStr := fmt.Sprintf("%v", rowDataFromFind[2])
		currentStatus = fmt.Sprintf("%v", rowDataFromFind[1])
		parsedStartTime, errTime := time.ParseInLocation("15:04:05", startTimeSheetStr, kyivLocation)
		if errTime == nil {
			year, month, day := localEndTime.Date()
			startTimeInKyiv = time.Date(year, month, day, parsedStartTime.Hour(), parsedStartTime.Minute(), parsedStartTime.Second(), 0, kyivLocation)
			if (currentStatus == "Розпочато" || currentStatus == "Робочий") && localEndTime.After(startTimeInKyiv) {
				duration = localEndTime.Sub(startTimeInKyiv)
			} else { log.Printf("Робота для ChatID %d на %s (%s) не була в статусі 'Розпочато' або час завершення некоректний.", chatID, todayStr, currentStatus) }
		} else { log.Printf("Не вдалося розпарсити час початку '%s' з таблиці для ChatID %d на %s: %v", startTimeSheetStr, chatID, todayStr, errTime) }
	} else { log.Printf("Недостатньо даних у рядку %d для розрахунку тривалості.", actualSheetRowIndex) }

	durationStr := FormatDuration(duration)
	newStatus := "Завершено"
	updateRangeDE := fmt.Sprintf("%s!D%d:E%d", workLogSheetName, actualSheetRowIndex, actualSheetRowIndex)
	valueRangeDE := &sheets.ValueRange{Values: [][]interface{}{{endTimeStr, durationStr}}}
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRangeDE, valueRangeDE).ValueInputOption("USER_ENTERED").Do()
	if err != nil { log.Printf("Помилка оновлення Часу Кінця/Тривалості в рядку %d на '%s': %v", actualSheetRowIndex, workLogSheetName, err) }

	statusUpdateRange := fmt.Sprintf("%s!B%d", workLogSheetName, actualSheetRowIndex)
	statusValueRange := &sheets.ValueRange{Values: [][]interface{}{{newStatus}}}
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).ValueInputOption("USER_ENTERED").Do()
	if err != nil { return zeroDuration, fmt.Errorf("не вдалося оновити запис про завершення роботи: %w", err) }

	log.Printf("Рядок %d для дати %s успішно оновлено (роботу завершено, тривалість: %s).", actualSheetRowIndex, todayStr, durationStr)
	return duration, nil
}

func LogDayOff(srv *sheets.Service, spreadsheetID string, workLogSheetName string, chatID int64, dateToLog time.Time) error {
	localDate := dateToLog.In(kyivLocation)
	dateStr := localDate.Format("2006-01-02")
	log.Printf("Логування вихідного дня для ChatID %d на %s (Аркуш: %s)", chatID, dateStr, workLogSheetName)

	readRange := fmt.Sprintf("%s!A:A", workLogSheetName)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	var sheetValues [][]interface{}
	if err != nil { log.Printf("Помилка читання дат з '%s': %v.", workLogSheetName, err) } else { sheetValues = resp.Values }
	
	rowIndexInValues, _ := findRowByDate(sheetValues, dateStr)
	actualSheetRowIndex := -1
	if rowIndexInValues != -1 { actualSheetRowIndex = rowIndexInValues + 1 }

	rowData := []interface{}{dateStr, "Вихідний", "", "", ""}
	if actualSheetRowIndex != -1 {
		log.Printf("Знайдено рядок %d для дати %s. Оновлення статусу на 'Вихідний'...", actualSheetRowIndex, dateStr)
		updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, actualSheetRowIndex, actualSheetRowIndex)
		valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}
		_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).ValueInputOption("USER_ENTERED").Do()
		if err != nil { return fmt.Errorf("не вдалося оновити запис про вихідний день: %w", err) }
		log.Printf("Рядок %d для дати %s успішно оновлено (статус 'Вихідний').", actualSheetRowIndex, dateStr)
	} else {
		log.Printf("Не знайдено рядка для дати %s. Додавання нового запису 'Вихідний'...", dateStr)
		appendRange := fmt.Sprintf("%s!A:E", workLogSheetName)
		valueRange := &sheets.ValueRange{Values: [][]interface{}{rowData}}
		_, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()
		if err != nil { return fmt.Errorf("не вдалося додати запис про вихідний день: %w", err) }
		log.Printf("Новий рядок для дати %s успішно додано (статус 'Вихідний').", dateStr)
	}
	return nil
}


// CountWorkingDaysInRange підраховує кількість робочих днів
func CountWorkingDaysInRange(srv *sheets.Service, spreadsheetID string, workLogSheetName string, startDate, endDate time.Time) (int, error) {
	// Використовуємо workLogSheetName
	readRange := fmt.Sprintf("%s!A:B", workLogSheetName) 
	log.Printf("Підрахунок робочих днів: читання '%s' для діапазону %s - %s",
		readRange, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання даних з аркуша '%s': %v", workLogSheetName, err)
		return 0, fmt.Errorf("не вдалося прочитати робочий графік: %w", err)
	}

	dayStatusMap := make(map[string]string)
	if len(resp.Values) > 0 {
		for i, row := range resp.Values {
			if i == 0 && (len(row) > 0 && (fmt.Sprintf("%v", row[0]) == "Дата" || fmt.Sprintf("%v", row[0]) == "Date")) { continue }
			if len(row) >= 2 { dayStatusMap[fmt.Sprintf("%v", row[0])] = fmt.Sprintf("%v", row[1]) }
		}
	}
	log.Printf("Мапа статусів з аркуша '%s': %v", workLogSheetName, dayStatusMap)

	workingDays := 0
	currentDay := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, kyivLocation)
	lastDay := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, kyivLocation)

	for !currentDay.After(lastDay) {
		dateStr := currentDay.Format("2006-01-02")
		status, exists := dayStatusMap[dateStr]
		isWorkingDay := true
		if exists && status == "Вихідний" { isWorkingDay = false }
		if isWorkingDay { workingDays++; log.Printf("День %s вважається робочим (статус: '%s', існує: %t)", dateStr, status, exists) } else { log.Printf("День %s вважається ВИХІДНИМ (статус: '%s')", dateStr, status) }
		currentDay = currentDay.AddDate(0, 0, 1)
	}

	log.Printf("Знайдено %d робочих днів у діапазоні %s - %s", workingDays, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	return workingDays, nil
}

// getMotivation залишається без змін
func getMotivation() string { /* ... */ 
	hour := getCurrentTimeInKyiv().Hour() 
	switch {
	case hour < 12: return "Почни цей день потужно — результат не забариться!"
	case hour < 18: return "Тримай темп, ти вже ближче до мети!"
	default: return "Завершуй день із гордістю за зроблене!"
	}
}
