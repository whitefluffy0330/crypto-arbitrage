package sheets

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"
	// Імпорт "strings" було видалено, оскільки він не використовувався

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
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
	SetDate      time.Time // Має бути в UTC при збереженні, але можемо передавати/читати в локальному
}

// ---- Допоміжні функції ----

var kyivLocation *time.Location // Глобальна змінна для часової зони Києва

func init() {
	// Ініціалізуємо часову зону один раз при завантаженні пакета
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Printf("Критична помилка: не вдалося завантажити часову зону Europe/Kyiv: %v. Буде використано UTC.", err)
		kyivLocation = time.UTC // Використовуємо UTC як запасний варіант
	} else {
		kyivLocation = loc
		log.Println("Часову зону Europe/Kyiv успішно завантажено.")
	}
}

// getCurrentTimeInKyiv повертає поточний час у зоні Europe/Kyiv
func getCurrentTimeInKyiv() time.Time {
	return time.Now().In(kyivLocation)
}

// findRowIndexByDate знаходить індекс рядка (1-based) на аркуші за датою в першій колонці (A).
// Повертає індекс рядка або -1, якщо не знайдено.
// Припускає, що sheetData містить дані, включаючи заголовок у першому рядку (індекс 0).
func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) int {
	if len(sheetData) < 2 { // Потрібен хоча б заголовок і один рядок даних
		return -1
	}
	// Шукаємо з другого рядка (індекс 1), бо перший - заголовок
	for i := 1; i < len(sheetData); i++ {
		row := sheetData[i]
		if len(row) > 0 {
			if fmt.Sprintf("%v", row[0]) == dateToFind {
				return i + 1 // Повертаємо 1-based індекс рядка на аркуші
			}
		}
	}
	return -1 // Не знайдено
}

// FormatDuration публічна функція для форматування тривалості
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
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
	readRange := "Звіт!A2:E2" // Читаємо конкретний рядок
	var data SheetRowData
	var err error

	log.Printf("Спроба читання даних з Google Sheets: SpreadsheetID=%s, Range=%s", spreadsheetID, readRange)
	resp, errGet := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if errGet != nil {
		log.Printf("Не вдалося отримати дані з Google Sheets (GenerateProgressReport): %v", errGet)
		return data, fmt.Errorf("помилка отримання даних з Google Sheets: %w", errGet)
	}

	if len(resp.Values) < 1 || len(resp.Values[0]) < 5 {
		errMsg := fmt.Sprintf("недостатньо даних у діапазоні %s", readRange)
		log.Println(errMsg)
		return data, fmt.Errorf(errMsg)
	}

	row := resp.Values[0]
	// Обробка помилок парсингу покращена - записуємо 0 або "" у разі помилки
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) }
	if len(row) > 1 {
		incomeStr := fmt.Sprintf("%v", row[1])
		data.Income, err = strconv.ParseFloat(incomeStr, 64)
		if err != nil { log.Printf("Помилка парсингу доходу '%s': %v. Встановлено 0.", incomeStr, err); data.Income = 0 }
	}
	if len(row) > 2 {
		sheetGoalStr := fmt.Sprintf("%v", row[2])
		data.SheetGoal, err = strconv.ParseFloat(sheetGoalStr, 64)
		if err != nil { log.Printf("Помилка парсингу мети з таблиці '%s': %v. Встановлено 0.", sheetGoalStr, err); data.SheetGoal = 0 }
	}
	if len(row) > 3 {
		sheetDaysLeftStr := fmt.Sprintf("%v", row[3])
		data.SheetDaysLeft, err = strconv.Atoi(sheetDaysLeftStr)
		if err != nil { log.Printf("Помилка парсингу 'днів залишилося' з таблиці '%s': %v. Встановлено 0.", sheetDaysLeftStr, err); data.SheetDaysLeft = 0 }
	}
	if len(row) > 4 {
		sheetReqDailyStr := fmt.Sprintf("%v", row[4])
		data.SheetReqDaily, err = strconv.ParseFloat(sheetReqDailyStr, 64)
		if err != nil { log.Printf("Помилка парсингу 'потрібно щодня' з таблиці '%s': %v. Встановлено 0.", sheetReqDailyStr, err); data.SheetReqDaily = 0 }
	}
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
		goalData.SetDate.In(kyivLocation).Format("2006-01-02"), // Дата у форматі РРРР-ММ-ДД (локальна)
		"Активна", goalData.OriginalText, "")

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

// UpdateGoalStatusInSheet знаходить останню активну ціль користувача та оновлює її статус і дату закриття
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

	targetRowIndex := -1 // 1-based індекс рядка на аркуші
	if len(resp.Values) > 1 { // Перевіряємо, чи є хоча б один рядок даних крім заголовка
		// Шукаємо ЗНИЗУ ВГОРУ, щоб знайти останню активну ціль
		for i := len(resp.Values) - 1; i >= 1; i-- { // Починаємо з кінця, ігноруємо i=0 (заголовок)
			row := resp.Values[i]
			if len(row) >= 6 { // Потрібні колонки A та F
				rowChatIDStr := fmt.Sprintf("%v", row[0])
				rowStatusStr := fmt.Sprintf("%v", row[5])
				rowChatID, errChatID := strconv.ParseInt(rowChatIDStr, 10, 64)

				if errChatID == nil && rowChatID == chatID && rowStatusStr == "Активна" {
					targetRowIndex = i + 1 // Номер рядка в Google Sheets (1-based)
					break
				}
			}
		}
	}

	if targetRowIndex == -1 {
		log.Printf("Не знайдено активної цілі для ChatID %d на аркуші '%s' для оновлення статусу.", chatID, sheetName)
		return fmt.Errorf("не знайдено активної цілі для оновлення") // Повертаємо помилку, бо нема чого оновлювати
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
	closedDateValueRange := &sheets.ValueRange{ Values: [][]interface{}{{closedDate.In(kyivLocation).Format("2006-01-02")}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, closedDateUpdateRange, closedDateValueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення дати закриття цілі на аркуші '%s' для ChatID %d (рядок %d): %v", sheetName, chatID, targetRowIndex, err)
		// Не повертаємо помилку тут, оскільки статус вже оновлено, але логуємо
		// Можна повернути помилку, якщо це критично: return fmt.Errorf("не вдалося оновити дату закриття цілі: %w", err)
	}

	log.Printf("Статус цілі для ChatID %d (рядок %d) на аркуші '%s' успішно оновлено на '%s'", chatID, targetRowIndex, sheetName, newStatus)
	return nil
}

// --- Функції для роботи з аркушем "РобочийГрафік" ---
const workLogSheetName = "РобочийГрафік"

func LogWorkStart(srv *sheets.Service, spreadsheetID string, chatID int64, startTime time.Time) error {
	localStartTime := startTime.In(kyivLocation) 
	todayStr := localStartTime.Format("2006-01-02")
	startTimeStr := localStartTime.Format("15:04:05")

	log.Printf("Логування початку роботи для ChatID %d на %s, час: %s (Europe/Kyiv)", chatID, todayStr, startTimeStr)

	readRange := fmt.Sprintf("%s!A:A", workLogSheetName) 
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання дат з '%s': %v. Спробуємо додати новий рядок.", workLogSheetName, err)
	}
	rowIndex := findRowIndexByDate(resp.Values, todayStr) 

	rowData := []interface{}{todayStr, "Розпочато", startTimeStr, "", ""} 

	if rowIndex != -1 {
		log.Printf("Знайдено рядок %d для дати %s. Оновлення...", rowIndex, todayStr)
		updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, rowIndex, rowIndex)
		valueRange := &sheets.ValueRange{ Values: [][]interface{}{rowData} }
		_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).ValueInputOption("USER_ENTERED").Do()
		if err != nil {
			log.Printf("Помилка оновлення рядка %d на '%s': %v", rowIndex, workLogSheetName, err)
			return fmt.Errorf("не вдалося оновити запис про початок роботи: %w", err)
		}
		log.Printf("Рядок %d для дати %s успішно оновлено (роботу розпочато).", rowIndex, todayStr)
	} else {
		log.Printf("Не знайдено рядка для дати %s. Додавання нового...", todayStr)
		appendRange := fmt.Sprintf("%s!A:E", workLogSheetName)
		valueRange := &sheets.ValueRange{ Values: [][]interface{}{rowData} }
		_, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()
		if err != nil {
			log.Printf("Помилка додавання рядка на '%s': %v", workLogSheetName, err)
			return fmt.Errorf("не вдалося додати запис про початок роботи: %w", err)
		}
		log.Printf("Новий рядок для дати %s успішно додано (роботу розпочато).", todayStr)
	}
	return nil
}

func LogWorkStop(srv *sheets.Service, spreadsheetID string, chatID int64, endTime time.Time) (time.Duration, error) {
	localEndTime := endTime.In(kyivLocation) 
	todayStr := localEndTime.Format("2006-01-02")
	endTimeStr := localEndTime.Format("15:04:05")
	log.Printf("Логування завершення роботи для ChatID %d на %s, час: %s (Europe/Kyiv)", chatID, todayStr, endTimeStr)
	zeroDuration := time.Duration(0)

	readRange := fmt.Sprintf("%s!A:C", workLogSheetName) 
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання даних з '%s' для завершення роботи: %v", workLogSheetName, err)
		return zeroDuration, fmt.Errorf("не вдалося прочитати дані для завершення роботи: %w", err)
	}
	// Шукаємо ЗНИЗУ ВГОРУ, щоб знайти останній запис за сьогодні
	rowIndex := -1
	if len(resp.Values) > 0 {
		for i := len(resp.Values) - 1; i >= 1; i-- { // Починаємо з кінця, ігноруємо заголовок (i=0)
			row := resp.Values[i]
			if len(row) > 0 {
				if fmt.Sprintf("%v", row[0]) == todayStr {
					rowIndex = i + 1 // 1-based index
					break
				}
			}
		}
	}

	if rowIndex == -1 {
		log.Printf("Не знайдено рядка для дати %s на '%s', щоб зафіксувати кінець роботи.", todayStr, workLogSheetName)
		return zeroDuration, fmt.Errorf("не знайдено запису про початок роботи за сьогодні")
	}

	var startTimeInKyiv time.Time
	var duration time.Duration = zeroDuration
	var currentStatus string = "Невідомо"
	
	rowDataIndex := rowIndex -1 
	if rowDataIndex >= 0 && rowDataIndex < len(resp.Values) && len(resp.Values[rowDataIndex]) >= 3 {
		startTimeSheetStr := fmt.Sprintf("%v", resp.Values[rowDataIndex][2]) 
		currentStatus = fmt.Sprintf("%v", resp.Values[rowDataIndex][1])
		
		parsedStartTime, errTime := time.ParseInLocation("15:04:05", startTimeSheetStr, kyivLocation)
		if errTime == nil {
			year, month, day := localEndTime.Date() 
			startTimeInKyiv = time.Date(year, month, day, parsedStartTime.Hour(), parsedStartTime.Minute(), parsedStartTime.Second(), 0, kyivLocation)
			if (currentStatus == "Розпочато" || currentStatus == "Робочий") && localEndTime.After(startTimeInKyiv) { 
				duration = localEndTime.Sub(startTimeInKyiv)
			} else {
				log.Printf("Робота для ChatID %d на %s (%s) не була в статусі 'Розпочато' або час завершення некоректний. Тривалість не розраховано.", chatID, todayStr, currentStatus)
			}
		} else {
			log.Printf("Не вдалося розпарсити час початку '%s' з таблиці для ChatID %d на %s: %v", startTimeSheetStr, chatID, todayStr, errTime)
		}
	} else {
		log.Printf("Недостатньо даних у рядку %d для розрахунку тривалості.", rowIndex)
	}

	durationStr := FormatDuration(duration)
	newStatus := "Завершено"
	
	updateRangeDE := fmt.Sprintf("%s!D%d:E%d", workLogSheetName, rowIndex, rowIndex)
	valueRangeDE := &sheets.ValueRange{ Values: [][]interface{}{{endTimeStr, durationStr}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRangeDE, valueRangeDE).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення Часу Кінця/Тривалості в рядку %d на '%s': %v", rowIndex, workLogSheetName, err)
	}

	statusUpdateRange := fmt.Sprintf("%s!B%d", workLogSheetName, rowIndex)
	statusValueRange := &sheets.ValueRange{ Values: [][]interface{}{{newStatus}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення статусу в рядку %d на '%s': %v", rowIndex, workLogSheetName, err)
		return zeroDuration, fmt.Errorf("не вдалося оновити запис про завершення роботи: %w", err) 
	}

	log.Printf("Рядок %d для дати %s успішно оновлено (роботу завершено, тривалість: %s).", rowIndex, todayStr, durationStr)
	return duration, nil
}

func LogDayOff(srv *sheets.Service, spreadsheetID string, chatID int64, dateToLog time.Time) error {
	localDate := dateToLog.In(kyivLocation) 
	dateStr := localDate.Format("2006-01-02")
	log.Printf("Логування вихідного дня для ChatID %d на %s (Europe/Kyiv)", chatID, dateStr)

	readRange := fmt.Sprintf("%s!A:A", workLogSheetName)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання дат з '%s': %v. Спробуємо додати новий рядок.", workLogSheetName, err)
	}
	rowIndex := findRowIndexByDate(resp.Values, dateStr)

	rowData := []interface{}{dateStr, "Вихідний", "", "", ""} 

	if rowIndex != -1 {
		log.Printf("Знайдено рядок %d для дати %s. Оновлення статусу на 'Вихідний'...", rowIndex, dateStr)
		updateRange := fmt.Sprintf("%s!A%d:E%d", workLogSheetName, rowIndex, rowIndex) 
		valueRange := &sheets.ValueRange{ Values: [][]interface{}{rowData} }
		_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRange, valueRange).ValueInputOption("USER_ENTERED").Do()
		if err != nil {
			log.Printf("Помилка оновлення рядка %d на '%s': %v", rowIndex, workLogSheetName, err)
			return fmt.Errorf("не вдалося оновити запис про вихідний день: %w", err)
		}
		log.Printf("Рядок %d для дати %s успішно оновлено (статус 'Вихідний').", rowIndex, dateStr)
	} else {
		log.Printf("Не знайдено рядка для дати %s. Додавання нового запису 'Вихідний'...", dateStr)
		appendRange := fmt.Sprintf("%s!A:E", workLogSheetName)
		valueRange := &sheets.ValueRange{ Values: [][]interface{}{rowData} }
		_, err = srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Do()
		if err != nil {
			log.Printf("Помилка додавання рядка на '%s': %v", workLogSheetName, err)
			return fmt.Errorf("не вдалося додати запис про вихідний день: %w", err)
		}
		log.Printf("Новий рядок для дати %s успішно додано (статус 'Вихідний').", dateStr)
	}
	return nil
}

// НОВА функція: GetActiveGoalFromSheet завантажує останню активну ціль для користувача з аркуша "МоїЦілі"
func GetActiveGoalFromSheet(srv *sheets.Service, spreadsheetID string, chatID int64) (FinancialGoalData, bool, error) {
	sheetName := "МоїЦілі"
	var goalData FinancialGoalData
	var found bool

	// Читаємо весь аркуш, щоб знайти останню активну ціль для цього chatID
	// A: ChatID, B: Amount, C: Currency, D: Days, E: SetDate, F: Status, G: OriginalText
	readRange := fmt.Sprintf("%s!A:G", sheetName) // Читаємо до OriginalText
	log.Printf("Пошук активної цілі для ChatID %d на аркуші '%s'", chatID, sheetName)

	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання даних з аркуша '%s' для пошуку активної цілі (ChatID %d): %v", sheetName, chatID, err)
		return goalData, false, fmt.Errorf("не вдалося прочитати дані цілей: %w", err)
	}

	if len(resp.Values) > 1 { // Має бути хоча б заголовок і рядок даних
		// Шукаємо знизу вгору
		for i := len(resp.Values) - 1; i >= 1; i-- { // Починаємо з кінця, ігноруємо заголовок (i=0)
			row := resp.Values[i]
			
			// Перевіряємо, чи достатньо колонок і чи збігається ChatID та статус
			if len(row) >= 7 { // Потрібні дані до колонки G (OriginalText) включно, F - Status
				rowChatIDStr := fmt.Sprintf("%v", row[0])
				rowStatusStr := fmt.Sprintf("%v", row[5]) // Колонка F - Status

				rowChatID, errChatID := strconv.ParseInt(rowChatIDStr, 10, 64)

				if errChatID == nil && rowChatID == chatID && rowStatusStr == "Активна" {
					// Знайшли активну ціль! Тепер парсимо її.
					amountStr := fmt.Sprintf("%v", row[1])
					currencyStr := fmt.Sprintf("%v", row[2])
					daysStr := fmt.Sprintf("%v", row[3])
					setDateStr := fmt.Sprintf("%v", row[4]) // Очікуємо "РРРР-ММ-ДД"
					originalTextStr := fmt.Sprintf("%v", row[6])

					goalData.Amount, _ = strconv.ParseFloat(amountStr, 64) // Ігноруємо помилки парсингу з таблиці тут
					goalData.Currency = currencyStr
					goalData.Days, _ = strconv.Atoi(daysStr)
					goalData.OriginalText = originalTextStr
					
					// Конвертуємо дату встановлення з формату "РРРР-ММ-ДД" у time.Time (в UTC)
					parsedSetDate, errDate := time.ParseInLocation("2006-01-02", setDateStr, kyivLocation)
					if errDate == nil {
						goalData.SetDate = parsedSetDate.UTC() // Зберігаємо в UTC
					} else {
						log.Printf("Помилка парсингу SetDate '%s' для ChatID %d: %v. Використовується час завантаження.", setDateStr, chatID, errDate)
						goalData.SetDate = time.Now().UTC() // Запасний варіант
					}
					
					found = true
					log.Printf("Знайдено активну ціль для ChatID %d на аркуші '%s': %+v", chatID, sheetName, goalData)
					break // Знайшли останню активну, виходимо
				}
			}
		}
	}

	if !found {
		log.Printf("Активну ціль для ChatID %d на аркуші '%s' не знайдено.", chatID, sheetName)
	}
	// Повертаємо помилку nil, оскільки відсутність цілі - це не помилка читання
	return goalData, found, nil 
}


// getMotivation (використовується у звіті)
func getMotivation() string {
	hour := getCurrentTimeInKyiv().Hour() 
	switch {
	case hour < 12:
		return "Почни цей день потужно — результат не забариться!"
	case hour < 18:
		return "Тримай темп, ти вже ближче до мети!"
	default:
		return "Завершуй день із гордістю за зроблене!"
	}
}
