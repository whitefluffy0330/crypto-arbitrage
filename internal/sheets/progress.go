package sheets

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"
	// "strings" // Видалено цей імпорт, оскільки він не використовувався

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
func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) int {
	if len(sheetData) == 0 {
		return -1
	}
	// Припускаємо, що Values[0] - це заголовок, шукаємо з i=1
	for i, row := range sheetData {
		if i == 0 { continue } // Пропускаємо заголовок (рядок 1 аркуша)
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
	var err error // Оголошуємо змінну err

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
	// Покращена обробка помилок парсингу
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) }
	if len(row) > 1 { 
		incomeStr := fmt.Sprintf("%v", row[1])
		data.Income, err = strconv.ParseFloat(incomeStr, 64)
		if err != nil { log.Printf("Помилка парсингу доходу '%s': %v", incomeStr, err) /* Можна встановити 0 */ }
	}
	if len(row) > 2 {
		sheetGoalStr := fmt.Sprintf("%v", row[2])
		data.SheetGoal, err = strconv.ParseFloat(sheetGoalStr, 64)
		if err != nil { log.Printf("Помилка парсингу мети з таблиці '%s': %v", sheetGoalStr, err) }
	}
	if len(row) > 3 {
		sheetDaysLeftStr := fmt.Sprintf("%v", row[3])
		data.SheetDaysLeft, err = strconv.Atoi(sheetDaysLeftStr)
		if err != nil { log.Printf("Помилка парсингу 'днів залишилося' з таблиці '%s': %v", sheetDaysLeftStr, err) }
	}
	if len(row) > 4 {
		sheetReqDailyStr := fmt.Sprintf("%v", row[4])
		data.SheetReqDaily, err = strconv.ParseFloat(sheetReqDailyStr, 64)
		if err != nil { log.Printf("Помилка парсингу 'потрібно щодня' з таблиці '%s': %v", sheetReqDailyStr, err) }
	}
	log.Printf("Дані з аркуша 'Звіт' успішно розпарсені: %+v", data)
	return data, nil // Повертаємо nil як помилку, якщо парсинг пройшов (навіть якщо були помилки окремих полів, ми їх залогували)
}

// AddGoalToSheet додає нову ціль на аркуш "МоїЦілі"
func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, chatID int64, goalData FinancialGoalData) error {
	sheetName := "МоїЦілі"
	log.Printf("Додавання цілі на аркуш '%s' для ChatID %d: %+v", sheetName, chatID, goalData)
	
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

	// Читаємо колонки ChatID (A) та Status (F), щоб знайти рядок
	readRange := fmt.Sprintf("%s!A:F", sheetName) 
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання даних з аркуша '%s' для оновлення статусу (ChatID %d): %v", sheetName, chatID, err)
		return fmt.Errorf("не вдалося прочитати дані для оновлення статусу: %w", err)
	}

	targetRowIndex := -1 
	if len(resp.Values) > 0 {
		for i := len(resp.Values) - 1; i >= 0; i-- { // Шукаємо знизу вгору
			row := resp.Values[i]
			// Припускаємо, що перший рядок (i=0) - це заголовок, пропускаємо його.
			// Також перевіряємо, що є достатньо колонок.
			if i == 0 || len(row) < 6 { continue } 
			
			rowChatIDStr := fmt.Sprintf("%v", row[0]) 
			rowStatusStr := fmt.Sprintf("%v", row[5]) 
			rowChatID, _ := strconv.ParseInt(rowChatIDStr, 10, 64)

			if rowChatID == chatID && rowStatusStr == "Активна" {
				targetRowIndex = i + 1 // 1-based індекс рядка на аркуші
				break 
			}
		}
	}

	if targetRowIndex == -1 {
		log.Printf("Не знайдено активної цілі для ChatID %d на аркуші '%s' для оновлення статусу.", chatID, sheetName)
		return nil // Не знайдено активної цілі - не є помилкою самої операції
	}

	// Оновлюємо статус (колонка F)
	statusUpdateRange := fmt.Sprintf("%s!F%d", sheetName, targetRowIndex)
	statusValueRange := &sheets.ValueRange{ Values: [][]interface{}{{newStatus}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення статусу цілі на аркуші '%s' для ChatID %d (рядок %d): %v", sheetName, chatID, targetRowIndex, err)
		return fmt.Errorf("не вдалося оновити статус цілі: %w", err)
	}

	// Оновлюємо дату закриття (колонка H)
	closedDateUpdateRange := fmt.Sprintf("%s!H%d", sheetName, targetRowIndex)
	closedDateValueRange := &sheets.ValueRange{ Values: [][]interface{}{{closedDate.Format("2006-01-02")}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, closedDateUpdateRange, closedDateValueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення дати закриття цілі на аркуші '%s' для ChatID %d (рядок %d): %v", sheetName, chatID, targetRowIndex, err)
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

	readRange := fmt.Sprintf("%s!A:A", workLogSheetName) 
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання дат з '%s': %v. Спробуємо додати новий рядок.", workLogSheetName, err)
		// Не повертаємо помилку, а спробуємо додати
	}
	rowIndex := findRowIndexByDate(resp.Values, todayStr) // findRowIndexByDate визначена вище

	startTimeStr := startTime.Format("15:04:05") 
	rowData := []interface{}{todayStr, "Розпочато", startTimeStr, "", ""} // A:Дата, B:Статус, C:Час Початку, D:Час Кінця, E:Тривалість

	if rowIndex != -1 {
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

	readRange := fmt.Sprintf("%s!A:C", workLogSheetName) // Читаємо Дата(A), Статус(B), ЧасПочатку(C)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання даних з '%s' для завершення роботи: %v", workLogSheetName, err)
		return zeroDuration, fmt.Errorf("не вдалося прочитати дані для завершення роботи: %w", err)
	}
	rowIndex := findRowIndexByDate(resp.Values, todayStr) // findRowIndexByDate визначена вище

	if rowIndex == -1 {
		log.Printf("Не знайдено рядка для дати %s на '%s', щоб зафіксувати кінець роботи.", todayStr, workLogSheetName)
		return zeroDuration, fmt.Errorf("не знайдено запису про початок роботи за сьогодні")
	}

	var startTimeStr string
	var startTime time.Time
	var duration time.Duration = zeroDuration
	var currentStatus string = "Невідомо" // Статус за замовчуванням
	
	rowDataIndex := rowIndex -1 // 0-based індекс для доступу до resp.Values
	if rowDataIndex >= 0 && rowDataIndex < len(resp.Values) && len(resp.Values[rowDataIndex]) >= 3 {
		startTimeStr = fmt.Sprintf("%v", resp.Values[rowDataIndex][2]) 
		currentStatus = fmt.Sprintf("%v", resp.Values[rowDataIndex][1])
		
		startTimeParsed, errTime := time.Parse("2006-01-02 15:04:05", todayStr+" "+startTimeStr)
		if errTime == nil {
			startTime = startTimeParsed
			if currentStatus == "Розпочато" || currentStatus == "Робочий" { 
				duration = endTime.Sub(startTime)
			} else {
				log.Printf("Робота для ChatID %d на %s вже була завершена або інший статус (%s). Тривалість не розраховано.", chatID, todayStr, currentStatus)
			}
		} else {
			log.Printf("Не вдалося розпарсити час початку '%s' для ChatID %d на %s: %v", startTimeStr, chatID, todayStr, errTime)
		}
	} else {
		log.Printf("Недостатньо даних у рядку %d для розрахунку тривалості.", rowIndex)
	}

	durationStr := formatDuration(duration) // formatDuration визначена вище
	newStatus := "Завершено"
	// Оновлюємо D (Час Кінця) та E (Тривалість)
	updateRange := fmt.Sprintf("%s!D%d:E%d", workLogSheetName, rowIndex, rowIndex)
	valueRange := &sheets.ValueRange{ Values: [][]interface{}{{endTimeStr, durationStr}} }
	
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення D:E рядка %d на '%s' для завершення роботи: %v", rowIndex, workLogSheetName, err)
		return zeroDuration, fmt.Errorf("не вдалося оновити запис про час завершення роботи: %w", err)
	}

	// Окремо оновлюємо статус (B)
	statusUpdateRange := fmt.Sprintf("%s!B%d", workLogSheetName, rowIndex)
	statusValueRange := &sheets.ValueRange{ Values: [][]interface{}{{newStatus}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення статусу (B) рядка %d на '%s' для завершення роботи: %v", rowIndex, workLogSheetName, err)
		// Продовжуємо, але повертаємо тривалість, оскільки час кінця міг записатися
	}


	log.Printf("Рядок %d для дати %s успішно оновлено (роботу завершено, тривалість: %s).", rowIndex, todayStr, durationStr)
	return duration, nil
}


// LogDayOff знаходить/додає рядок для вказаної дати та встановлює статус "Вихідний"
func LogDayOff(srv *sheets.Service, spreadsheetID string, chatID int64, dateStr string) error {
	log.Printf("Логування вихідного дня для ChatID %d на %s", chatID, dateStr)

	readRange := fmt.Sprintf("%s!A:A", workLogSheetName)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання дат з '%s': %v. Спробуємо додати новий рядок.", workLogSheetName, err)
	}
	rowIndex := findRowIndexByDate(resp.Values, dateStr) // findRowIndexByDate визначена вище

	rowData := []interface{}{dateStr, "Вихідний", "", "", ""} // A:Дата, B:Статус, C:Час Початку, D:Час Кінця, E:Тривалість

	if rowIndex != -1 {
		log.Printf("Знайдено рядок %d для дати %s. Оновлення статусу на 'Вихідний'...", rowIndex, dateStr)
		updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, rowIndex, rowIndex) // Оновлюємо весь рядок, щоб очистити час
		valueRange := &sheets.ValueRange{ Values: [][]interface{}{rowData} }
		_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).
			ValueInputOption("USER_ENTERED").Do()
		if err != nil {
			log.Printf("Помилка оновлення рядка %d на '%s': %v", rowIndex, workLogSheetName, err)
			return fmt.Errorf("не вдалося оновити запис про вихідний день: %w", err)
		}
		log.Printf("Рядок %d для дати %s успішно оновлено (статус 'Вихідний').", rowIndex, dateStr)
	} else {
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
