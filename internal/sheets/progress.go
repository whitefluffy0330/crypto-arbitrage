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

// --- Допоміжні функції ---

var kyivLocation *time.Location // Глобальна змінна для часової зони Києва

func init() {
	// Ініціалізуємо часову зону один раз при завантаженні пакета
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.Printf("Критична помилка: не вдалося завантажити часову зону Europe/Kyiv: %v. Буде використано UTC.", err)
		kyivLocation = time.UTC // Використовуємо UTC як запасний варіант
	} else {
		kyivLocation = loc
	}
}

// getCurrentTimeInKyiv повертає поточний час у зоні Europe/Kyiv
func getCurrentTimeInKyiv() time.Time {
	return time.Now().In(kyivLocation)
}

func findRowIndexByDate(sheetData [][]interface{}, dateToFind string) int {
	if len(sheetData) == 0 {
		return -1
	}
	for i, row := range sheetData {
		if i == 0 { continue } 
		if len(row) > 0 {
			if fmt.Sprintf("%v", row[0]) == dateToFind {
				return i + 1 
			}
		}
	}
	return -1
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	return fmt.Sprintf("%dh %dm", h, m)
}

// --- Функції роботи з Google Sheets API ---
// NewService, GenerateProgressReport, AddGoalToSheet, UpdateGoalStatusInSheet залишаються без змін у цій ітерації
// (якщо тільки GenerateProgressReport не мав би показувати дати/час з таблиці теж у локальному часі, але це складніше,
// бо ми не знаємо часовий пояс даних, що вже є в таблиці)

// NewService ... (код без змін)
func NewService(credentialsJSON []byte) (*sheets.Service, error) {
	ctx := context.Background()
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(credentialsJSON))
	if err != nil {
		return nil, fmt.Errorf("не вдалося створити клієнт Sheets: %w", err)
	}
	return srv, nil
}

// GenerateProgressReport ... (код без змін, але пам'ятайте, що SetDate для цілі встановлюється в UTC)
func GenerateProgressReport(srv *sheets.Service, spreadsheetID string) (SheetRowData, error) {
	readRange := "Звіт!A2:E2"
	var data SheetRowData
	var err error

	log.Printf("Спроба читання даних з Google Sheets: SpreadsheetID=%s, Range=%s", spreadsheetID, readRange)
	resp, errGet := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do() // Змінено ім'я змінної помилки
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
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) }
	if len(row) > 1 { 
		incomeStr := fmt.Sprintf("%v", row[1])
		data.Income, err = strconv.ParseFloat(incomeStr, 64)
		if err != nil { log.Printf("Помилка парсингу доходу '%s': %v", incomeStr, err) }
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
	return data, nil
}


// AddGoalToSheet ... (код SetDate використовує time.Now().UTC() - це нормально для зберігання, але відображатимемо в локальному часі)
func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, chatID int64, goalData FinancialGoalData) error {
	sheetName := "МоїЦілі"
	log.Printf("Додавання цілі на аркуш '%s' для ChatID %d: %+v", sheetName, chatID, goalData)
	
	var row []interface{}
	row = append(row, chatID, goalData.Amount, goalData.Currency, goalData.Days,
		goalData.SetDate.In(kyivLocation).Format("2006-01-02"), // Запис дати у локальному часі Києва
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

// UpdateGoalStatusInSheet ... (closedDate тепер теж буде в локальному часі)
func UpdateGoalStatusInSheet(srv *sheets.Service, spreadsheetID string, chatID int64, newStatus string, closedDate time.Time) error {
	sheetName := "МоїЦілі"
	log.Printf("Оновлення статусу цілі на '%s' на аркуші '%s' для ChatID %d", newStatus, sheetName, chatID)

	readRange := fmt.Sprintf("%s!A:F", sheetName) 
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання даних з аркуша '%s' для оновлення статусу (ChatID %d): %v", sheetName, chatID, err)
		return fmt.Errorf("не вдалося прочитати дані для оновлення статусу: %w", err)
	}

	targetRowIndex := -1 
	if len(resp.Values) > 0 {
		for i := len(resp.Values) - 1; i >= 0; i-- { 
			row := resp.Values[i]
			if i == 0 || len(row) < 6 { continue } 
			
			rowChatIDStr := fmt.Sprintf("%v", row[0]) 
			rowStatusStr := fmt.Sprintf("%v", row[5]) 
			rowChatID, _ := strconv.ParseInt(rowChatIDStr, 10, 64)

			if rowChatID == chatID && rowStatusStr == "Активна" {
				targetRowIndex = i + 1 
				break 
			}
		}
	}

	if targetRowIndex == -1 {
		log.Printf("Не знайдено активної цілі для ChatID %d на аркуші '%s' для оновлення статусу.", chatID, sheetName)
		return nil 
	}

	statusUpdateRange := fmt.Sprintf("%s!F%d", sheetName, targetRowIndex)
	statusValueRange := &sheets.ValueRange{ Values: [][]interface{}{{newStatus}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення статусу цілі: %v", err)
		return fmt.Errorf("не вдалося оновити статус цілі: %w", err)
	}

	closedDateUpdateRange := fmt.Sprintf("%s!H%d", sheetName, targetRowIndex)
	// Записуємо дату закриття у локальному часі Києва
	closedDateValueRange := &sheets.ValueRange{ Values: [][]interface{}{{closedDate.In(kyivLocation).Format("2006-01-02")}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, closedDateUpdateRange, closedDateValueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення дати закриття цілі: %v", err)
		return fmt.Errorf("не вдалося оновити дату закриття цілі: %w", err)
	}

	log.Printf("Статус цілі для ChatID %d (рядок %d) на аркуші '%s' успішно оновлено на '%s'", chatID, targetRowIndex, sheetName, newStatus)
	return nil
}


// --- НОВІ Функції для роботи з аркушем "РобочийГрафік" ---
const workLogSheetName = "РобочийГрафік"

// LogWorkStart знаходить/додає рядок для сьогоднішньої дати та записує час початку у Europe/Kyiv
func LogWorkStart(srv *sheets.Service, spreadsheetID string, chatID int64, startTime time.Time) error {
	localStartTime := startTime.In(kyivLocation) // Конвертуємо в Europe/Kyiv
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

// LogWorkStop знаходить рядок для сьогодні та записує час кінця і тривалість у Europe/Kyiv
func LogWorkStop(srv *sheets.Service, spreadsheetID string, chatID int64, endTime time.Time) (time.Duration, error) {
	localEndTime := endTime.In(kyivLocation) // Конвертуємо в Europe/Kyiv
	todayStr := localEndTime.Format("2006-01-02")
	endTimeStr := localEndTime.Format("15:04:05")
	log.Printf("Логування завершення роботи для ChatID %d на %s, час: %s (Europe/Kyiv)", chatID, todayStr, endTimeStr)
	zeroDuration := time.Duration(0)

	readRange := fmt.Sprintf("%s!A:C", workLogSheetName)
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання даних з '%s': %v", workLogSheetName, err)
		return zeroDuration, fmt.Errorf("не вдалося прочитати дані: %w", err)
	}
	rowIndex := findRowIndexByDate(resp.Values, todayStr)

	if rowIndex == -1 {
		log.Printf("Не знайдено рядка для дати %s на '%s'.", todayStr, workLogSheetName)
		return zeroDuration, fmt.Errorf("не знайдено запису про початок роботи")
	}

	var startTimeStr string
	var startTimeInKyiv time.Time
	var duration time.Duration = zeroDuration
	var currentStatus string = "Невідомо"
	
	rowDataIndex := rowIndex -1 
	if rowDataIndex >= 0 && rowDataIndex < len(resp.Values) && len(resp.Values[rowDataIndex]) >= 3 {
		startTimeSheetStr := fmt.Sprintf("%v", resp.Values[rowDataIndex][2]) // Час початку з таблиці
		currentStatus = fmt.Sprintf("%v", resp.Values[rowDataIndex][1])
		
		// Парсимо час початку, припускаючи, що він вже в Europe/Kyiv форматі з таблиці
		// або ми можемо припустити, що він був записаний як UTC, і конвертувати.
		// Для простоти, припускаємо, що час у таблиці вже "правильний" для розрахунку.
		// Краще: записувати час початку завжди як UTC, а тут отримувати startTime як UTC і endTime як UTC,
		// потім розраховувати тривалість, а вже для відображення форматувати в локальний час.
		// Поточна логіка: startTime береться з getCurrentTimeInKyiv(), отже, воно локальне.
		// endTime передається як time.Now() з commands.go, яке ми також зробимо локальним.

		// Завантажуємо час початку з таблиці (який вже має бути у форматі HH:MM:SS для Europe/Kyiv)
		// і поєднуємо з сьогоднішньою датою для коректного розрахунку Sub
		parsedStartTime, errTime := time.ParseInLocation("15:04:05", startTimeSheetStr, kyivLocation)
		if errTime == nil {
			// Поєднуємо дату з розпарсеним часом
			year, month, day := localEndTime.Date() // Беремо дату з localEndTime для консистентності
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

	durationStr := formatDuration(duration)
	newStatus := "Завершено"
	
	// Оновлюємо колонку D (Час Кінця) та E (Тривалість)
	updateRangeDE := fmt.Sprintf("%s!D%d:E%d", workLogSheetName, rowIndex, rowIndex)
	valueRangeDE := &sheets.ValueRange{ Values: [][]interface{}{{endTimeStr, durationStr}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, updateRangeDE, valueRangeDE).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення Часу Кінця/Тривалості в рядку %d на '%s': %v", rowIndex, workLogSheetName, err)
		// Можемо повернути помилку, або лише залогувати і спробувати оновити статус
	}

	// Оновлюємо Статус (колонка B)
	statusUpdateRange := fmt.Sprintf("%s!B%d", workLogSheetName, rowIndex)
	statusValueRange := &sheets.ValueRange{ Values: [][]interface{}{{newStatus}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення статусу в рядку %d на '%s': %v", rowIndex, workLogSheetName, err)
		return zeroDuration, fmt.Errorf("не вдалося оновити запис про завершення роботи: %w", err) // Повертаємо помилку, якщо статус не оновився
	}

	log.Printf("Рядок %d для дати %s успішно оновлено (роботу завершено, тривалість: %s).", rowIndex, todayStr, durationStr)
	return duration, nil
}

// LogDayOff знаходить/додає рядок для вказаної дати (в Europe/Kyiv) та встановлює статус "Вихідний"
func LogDayOff(srv *sheets.Service, spreadsheetID string, chatID int64, dateToLog time.Time) error {
	localDate := dateToLog.In(kyivLocation) // Конвертуємо в Europe/Kyiv
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

// getMotivation (залишається без змін)
func getMotivation() string {
	hour := time.Now().In(kyivLocation).Hour() // Використовуємо локальний час для мотивації
	switch {
	case hour < 12:
		return "Почни цей день потужно — результат не забариться!"
	case hour < 18:
		return "Тримай темп, ти вже ближче до мети!"
	default:
		return "Завершуй день із гордістю за зроблене!"
	}
}
