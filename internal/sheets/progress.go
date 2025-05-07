package sheets

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4" // Використовуємо sheets для прямого посилання на типи API
)

// Дозвіл на читання та запис
const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets"

// ---- Структури ----

// SheetRowData структура для зберігання даних, прочитаних з одного рядка аркуша "Звіт"
type SheetRowData struct {
	Date          string
	Income        float64
	SheetGoal     float64
	SheetDaysLeft int
	SheetReqDaily float64
}

// FinancialGoalData структура для передачі даних цілі в функції роботи з таблицею.
type FinancialGoalData struct {
	Amount       float64
	Currency     string
	Days         int
	OriginalText string
	SetDate      time.Time
}

// ---- Допоміжні функції ----

// findRowIndexByDate знаходить індекс рядка (1-based) на аркуші за датою в першій колонці (A).
// Повертає індекс рядка або -1, якщо не знайдено.
// Припускає, що sheetData містить дані, починаючи з другого рядка аркуша (після заголовка).
func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) int {
	if len(sheetData) == 0 {
		return -1
	}
	// Якщо ваш readRange починався з A1 і містить заголовок, індекс буде i + 1.
	// Якщо readRange починався з A2 (без заголовка), індекс буде i + 2 (бо Sheets 1-based).
	// Ми читаємо A:A, тому Values[0] - це A1. Починаємо шукати з другого рядка (i=1).
	for i, row := range sheetData {
		if i == 0 { continue } // Пропускаємо заголовок (рядок 1)
		if len(row) > 0 {
			if fmt.Sprintf("%v", row[0]) == dateToFind {
				return i + 1 // Повертаємо 1-based індекс рядка
			}
		}
	}
	return -1 // Не знайдено
}

// formatDuration форматує тривалість у години та хвилини (наприклад, "2h 35m")
func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute) // Округлюємо до хвилин
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	return fmt.Sprintf("%dh %dm", h, m)
}


// --- Функції роботи з Google Sheets API ---

// NewService (ймовірно, не використовується)
func NewService(credentialsJSON []byte) (*sheets.Service, error) {
	ctx := context.Background()
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(credentialsJSON))
	if err != nil {
		return nil, fmt.Errorf("не вдалося створити клієнт Sheets: %w", err)
	}
	return srv, nil
}

// GenerateProgressReport читає дані з аркуша "Звіт"
func GenerateProgressReport(srv *sheets.Service, spreadsheetID string) (SheetRowData, error) {
	readRange := "Звіт!A2:E2"
	var data SheetRowData

	log.Printf("Спроба читання даних з Google Sheets: SpreadsheetID=%s, Range=%s", spreadsheetID, readRange)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Не вдалося отримати дані з Google Sheets (GenerateProgressReport): %v", err)
		return data, fmt.Errorf("помилка отримання даних з Google Sheets: %w", err)
	}

	if len(resp.Values) < 1 || len(resp.Values[0]) < 5 {
		errMsg := fmt.Sprintf("недостатньо даних у діапазоні %s", readRange)
		log.Println(errMsg)
		return data, fmt.Errorf(errMsg)
	}

	row := resp.Values[0]
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) }
	if len(row) > 1 { incomeStr := fmt.Sprintf("%v", row[1]); data.Income, _ = strconv.ParseFloat(incomeStr, 64) }
	if len(row) > 2 { sheetGoalStr := fmt.Sprintf("%v", row[2]); data.SheetGoal, _ = strconv.ParseFloat(sheetGoalStr, 64) }
	if len(row) > 3 { sheetDaysLeftStr := fmt.Sprintf("%v", row[3]); data.SheetDaysLeft, _ = strconv.Atoi(sheetDaysLeftStr) }
	if len(row) > 4 { sheetReqDailyStr := fmt.Sprintf("%v", row[4]); data.SheetReqDaily, _ = strconv.ParseFloat(sheetReqDailyStr, 64) }
	
	log.Printf("Дані з аркуша 'Звіт' успішно розпарсені: %+v", data)
	return data, nil
}

// AddGoalToSheet додає нову ціль на аркуш "МоїЦілі"
func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, chatID int64, goalData FinancialGoalData) error {
	sheetName := "МоїЦілі"
	log.Printf("Додавання цілі на аркуш '%s' для ChatID %d: %+v", sheetName, chatID, goalData)
	
	// A: ChatID, B: Amount, C: Currency, D: Days, E: SetDate, F: Status, G: OriginalText, H: ClosedDate
	var row []interface{}
	row = append(row, chatID, goalData.Amount, goalData.Currency, goalData.Days,
		goalData.SetDate.Format("2006-01-02"), "Активна", goalData.OriginalText, "")

	valueRange := &sheets.ValueRange{ Values: [][]interface{}{row} }
	appendRange := fmt.Sprintf("%s!A:H", sheetName)

	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
		ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()

	if err != nil {
		log.Printf("Помилка запису цілі на аркуш '%s' для ChatID %d: %v", sheetName, chatID, err)
		return fmt.Errorf("не вдалося записати ціль у таблицю: %w", err)
	}
	log.Printf("Ціль для ChatID %d успішно записана на аркуш '%s'", chatID, sheetName)
	return nil
}

// UpdateGoalStatusInSheet знаходить активну ціль користувача та оновлює її статус і дату закриття
func UpdateGoalStatusInSheet(srv *sheets.Service, spreadsheetID string, chatID int64, newStatus string, closedDate time.Time) error {
	sheetName := "МоїЦілі"
	log.Printf("Оновлення статусу цілі на '%s' на аркуші '%s' для ChatID %d", newStatus, sheetName, chatID)

	readRange := fmt.Sprintf("%s!A:F", sheetName) // Читаємо до колонки статусу F
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання даних з аркуша '%s' для оновлення статусу (ChatID %d): %v", sheetName, chatID, err)
		return fmt.Errorf("не вдалося прочитати дані для оновлення статусу: %w", err)
	}

	targetRowIndex := -1 // 1-based індекс рядка на аркуші
	if len(resp.Values) > 0 {
		// Шукаємо ЗНИЗУ ВГОРУ, щоб знайти останню активну ціль, якщо їх кілька
		for i := len(resp.Values) - 1; i >= 0; i-- {
			row := resp.Values[i]
			// Перевіряємо, чи це не заголовок (про всяк випадок) і чи достатньо колонок
			if i == 0 || len(row) < 6 { continue } 
			
			rowChatIDStr := fmt.Sprintf("%v", row[0]) // Колонка A - ChatID
			rowStatusStr := fmt.Sprintf("%v", row[5]) // Колонка F - Status
			rowChatID, _ := strconv.ParseInt(rowChatIDStr, 10, 64)

			if rowChatID == chatID && rowStatusStr == "Активна" {
				targetRowIndex = i + 1 // Номер рядка в Google Sheets (1-based)
				break // Знайшли останню активну, виходимо
			}
		}
	}

	if targetRowIndex == -1 {
		log.Printf("Не знайдено активної цілі для ChatID %d на аркуші '%s' для оновлення статусу.", chatID, sheetName)
		// Не повертаємо помилку, можливо користувач вже закрив ціль вручну або не мав активної
		return nil 
	}

	// Оновлюємо статус (колонка F)
	statusUpdateRange := fmt.Sprintf("%s!F%d", sheetName, targetRowIndex)
	statusValueRange := &sheets.ValueRange{ Values: [][]interface{}{{newStatus}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення статусу цілі на аркуші '%s' для ChatID %d (рядок %d): %v", sheetName, chatID, targetRowIndex, err)
		// Повертаємо помилку, бо не вдалося оновити статус
		return fmt.Errorf("не вдалося оновити статус цілі: %w", err)
	}

	// Оновлюємо дату закриття (колонка H)
	closedDateUpdateRange := fmt.Sprintf("%s!H%d", sheetName, targetRowIndex)
	closedDateValueRange := &sheets.ValueRange{ Values: [][]interface{}{{closedDate.Format("2006-01-02")}} } // Використовуємо формат РРРР-ММ-ДД
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, closedDateUpdateRange, closedDateValueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення дати закриття цілі на аркуші '%s' для ChatID %d (рядок %d): %v", sheetName, chatID, targetRowIndex, err)
		// Можна повернути помилку або лише залогувати, оскільки статус вже оновлено
		return fmt.Errorf("не вдалося оновити дату закриття цілі: %w", err)
	}

	log.Printf("Статус цілі для ChatID %d (рядок %d) на аркуші '%s' успішно оновлено на '%s'", chatID, targetRowIndex, sheetName, newStatus)
	return nil
}


// --- НОВІ Функції для роботи з аркушем "РобочийГрафік" ---

const workLogSheetName = "РобочийГрафік"

// LogWorkStart знаходить/додає рядок для сьогоднішньої дати та записує час початку
func LogWorkStart(srv *sheets.Service, spreadsheetID string, chatID int64, startTime time.Time) error {
	todayStr := startTime.Format("2006-01-02")
	log.Printf("Логування початку роботи для ChatID %d на %s", chatID, todayStr)

	// 1. Шукаємо рядок з сьогоднішньою датою
	readRange := fmt.Sprintf("%s!A:A", workLogSheetName) // Читаємо колонку дат
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання дат з '%s': %v", workLogSheetName, err)
		// Не критично, спробуємо додати новий рядок
	}
	rowIndex := findRowIndexByDate(resp.Values, todayStr)

	// Готуємо дані для оновлення/додавання
	// A:Дата, B:Статус, C:Час Початку, D:Час Кінця, E:Тривалість роботи
	startTimeStr := startTime.Format("15:04:05") // Формат HH:MM:SS
	rowData := []interface{}{todayStr, "Розпочато", startTimeStr, "", ""} 

	if rowIndex != -1 {
		// Знайдено рядок для сьогодні, оновлюємо його
		log.Printf("Знайдено рядок %d для дати %s. Оновлення...", rowIndex, todayStr)
		updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, rowIndex, rowIndex)
		valueRange := &sheets.ValueRange{ Values: [][]interface{}{rowData} }
		_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).
			ValueInputOption("USER_ENTERED").Do()
		if err != nil {
			log.Printf("Помилка оновлення рядка %d на '%s': %v", rowIndex, workLogSheetName, err)
			return fmt.Errorf("не вдалося оновити запис про початок роботи: %w", err)
		}
		log.Printf("Рядок %d для дати %s успішно оновлено (роботу розпочато).", rowIndex, todayStr)
	} else {
		// Не знайдено рядка для сьогодні, додаємо новий
		log.Printf("Не знайдено рядка для дати %s. Додавання нового...", todayStr)
		appendRange := fmt.Sprintf("%s!A:E", workLogSheetName)
		valueRange := &sheets.ValueRange{ Values: [][]interface{}{rowData} }
		_, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
			ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()
		if err != nil {
			log.Printf("Помилка додавання рядка на '%s': %v", workLogSheetName, err)
			return fmt.Errorf("не вдалося додати запис про початок роботи: %w", err)
		}
		log.Printf("Новий рядок для дати %s успішно додано (роботу розпочато).", todayStr)
	}

	return nil
}

// LogWorkStop знаходить рядок для сьогодні та записує час кінця і тривалість
func LogWorkStop(srv *sheets.Service, spreadsheetID string, chatID int64, endTime time.Time) (time.Duration, error) {
	todayStr := endTime.Format("2006-01-02")
	endTimeStr := endTime.Format("15:04:05")
	log.Printf("Логування завершення роботи для ChatID %d на %s", chatID, todayStr)
	zeroDuration := time.Duration(0)

	// 1. Шукаємо рядок з сьогоднішньою датою
	// Читаємо Дата(A), Статус(B), ЧасПочатку(C)
	readRange := fmt.Sprintf("%s!A:C", workLogSheetName)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання даних з '%s' для завершення роботи: %v", workLogSheetName, err)
		return zeroDuration, fmt.Errorf("не вдалося прочитати дані для завершення роботи: %w", err)
	}
	rowIndex := findRowIndexByDate(resp.Values, todayStr)

	if rowIndex == -1 {
		log.Printf("Не знайдено рядка для дати %s на '%s', щоб зафіксувати кінець роботи.", todayStr, workLogSheetName)
		return zeroDuration, fmt.Errorf("не знайдено запису про початок роботи за сьогодні")
	}

	// 2. Отримуємо час початку з таблиці та розраховуємо тривалість
	var startTimeStr string
	var startTime time.Time
	var duration time.Duration = zeroDuration
	var currentStatus string
	
	// resp.Values має індексацію з 0, rowIndex - 1-based індекс рядка на аркуші
	// Потрібен 0-based індекс для зрізу resp.Values
	rowDataIndex := rowIndex -1
	if rowDataIndex >= 0 && rowDataIndex < len(resp.Values) && len(resp.Values[rowDataIndex]) >= 3 {
		startTimeStr = fmt.Sprintf("%v", resp.Values[rowDataIndex][2]) // Колонка C
		currentStatus = fmt.Sprintf("%v", resp.Values[rowDataIndex][1]) // Колонка B
		
		// Намагаємося розпарсити час початку (припускаємо формат HH:MM:SS)
		// Додаємо сьогоднішню дату, щоб отримати повний time.Time
		startTimeParsed, errTime := time.Parse("2006-01-02 15:04:05", todayStr+" "+startTimeStr)
		if errTime == nil {
			startTime = startTimeParsed
			if currentStatus == "Розпочато" || currentStatus == "Робочий" { // Розраховуємо тривалість, тільки якщо робота була розпочата
				duration = endTime.Sub(startTime)
			} else {
				log.Printf("Робота для ChatID %d на %s вже була завершена або скасована раніше (статус: %s). Тривалість не розраховано.", chatID, todayStr, currentStatus)
			}
		} else {
			log.Printf("Не вдалося розпарсити час початку '%s' для ChatID %d на %s: %v", startTimeStr, chatID, todayStr, errTime)
			// Продовжуємо без розрахунку тривалості
		}
	} else {
		log.Printf("Недостатньо даних у рядку %d для розрахунку тривалості.", rowIndex)
	}

	// 3. Оновлюємо Статус(B), Час Кінця(D), Тривалість(E)
	durationStr := formatDuration(duration)
	newStatus := "Завершено"
	updateData := []interface{}{newStatus, startTimeStr, endTimeStr, durationStr} // Дані для B, C, D, E
	
	updateRange := fmt.Sprintf("%s!B%d:E%d", workLogSheetName, rowIndex, rowIndex)
	valueRange := &sheets.ValueRange{ Values: [][]interface{}{updateData} }
	
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення рядка %d на '%s' для завершення роботи: %v", rowIndex, workLogSheetName, err)
		return zeroDuration, fmt.Errorf("не вдалося оновити запис про завершення роботи: %w", err)
	}

	log.Printf("Рядок %d для дати %s успішно оновлено (роботу завершено, тривалість: %s).", rowIndex, todayStr, durationStr)
	return duration, nil
}

// LogDayOff знаходить/додає рядок для вказаної дати та встановлює статус "Вихідний"
func LogDayOff(srv *sheets.Service, spreadsheetID string, chatID int64, dateStr string) error {
	log.Printf("Логування вихідного дня для ChatID %d на %s", chatID, dateStr)

	// 1. Шукаємо рядок з датою
	readRange := fmt.Sprintf("%s!A:A", workLogSheetName)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання дат з '%s': %v", workLogSheetName, err)
	}
	rowIndex := findRowIndexByDate(resp.Values, dateStr)

	// Готуємо дані для оновлення/додавання
	rowData := []interface{}{dateStr, "Вихідний", "", "", ""} // Дата, Статус, очищуємо C, D, E

	if rowIndex != -1 {
		// Знайдено рядок, оновлюємо його
		log.Printf("Знайдено рядок %d для дати %s. Оновлення статусу на 'Вихідний'...", rowIndex, dateStr)
		updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, rowIndex, rowIndex)
		valueRange := &sheets.ValueRange{ Values: [][]interface{}{rowData} }
		_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).
			ValueInputOption("USER_ENTERED").Do()
		if err != nil {
			log.Printf("Помилка оновлення рядка %d на '%s': %v", rowIndex, workLogSheetName, err)
			return fmt.Errorf("не вдалося оновити запис про вихідний день: %w", err)
		}
		log.Printf("Рядок %d для дати %s успішно оновлено (статус 'Вихідний').", rowIndex, dateStr)
	} else {
		// Не знайдено рядка, додаємо новий
		log.Printf("Не знайдено рядка для дати %s. Додавання нового запису 'Вихідний'...", dateStr)
		appendRange := fmt.Sprintf("%s!A:E", workLogSheetName)
		valueRange := &sheets.ValueRange{ Values: [][]interface{}{rowData} }
		_, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
			ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()
		if err != nil {
			log.Printf("Помилка додавання рядка на '%s': %v", workLogSheetName, err)
			return fmt.Errorf("не вдалося додати запис про вихідний день: %w", err)
		}
		log.Printf("Новий рядок для дати %s успішно додано (статус 'Вихідний').", dateStr)
	}

	return nil
}

// getMotivation (залишається без змін)
// ...
func getMotivation() string {
	hour := time.Now().Hour()
	switch {
	case hour < 12:
		return "Почни цей день потужно — результат не забариться!"
	case hour < 18:
		return "Тримай темп, ти вже ближче до мети!"
	default:
		return "Завершуй день із гордістю за зроблене!"
	}
}
