package sheets

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4" // Використовуємо sheets для прямого посилання на типи API
)

// ЗМІНЕНО SCOPE: тепер дозволяє читання ТА ЗАПИС
const SpreadsheetsScope = "https://www.googleapis.com/auth/spreadsheets"

// SheetRowData структура для зберігання даних, прочитаних з одного рядка аркуша "Звіт"
type SheetRowData struct {
	Date          string
	Income        float64
	SheetGoal     float64
	SheetDaysLeft int
	SheetReqDaily float64
}

// FinancialGoalData структура для передачі даних цілі в функції роботи з таблицею.
// Ми можемо використовувати тип telegram.FinancialGoal, якщо уникнути циклічної залежності,
// але простіше передати необхідні поля.
type FinancialGoalData struct {
	Amount       float64
	Currency     string
	Days         int
	OriginalText string
	SetDate      time.Time
}

// NewService (ймовірно, не використовується)
func NewService(credentialsJSON []byte) (*sheets.Service, error) {
	ctx := context.Background()
	// Використовуємо sheets.NewService з імпорту google.golang.org/api/sheets/v4
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
		errMsg := fmt.Sprintf("недостатньо даних у діапазоні %s: отримано %d рядків (або недостатньо колонок)", readRange, len(resp.Values))
		log.Println(errMsg)
		return data, fmt.Errorf(errMsg)
	}

	row := resp.Values[0]
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) }
	if len(row) > 1 {
		incomeStr := fmt.Sprintf("%v", row[1])
		data.Income, _ = strconv.ParseFloat(incomeStr, 64) // Помилки парсингу тут проігноровані для простоти, але їх варто обробляти
	}
	if len(row) > 2 {
		sheetGoalStr := fmt.Sprintf("%v", row[2])
		data.SheetGoal, _ = strconv.ParseFloat(sheetGoalStr, 64)
	}
	if len(row) > 3 {
		sheetDaysLeftStr := fmt.Sprintf("%v", row[3])
		data.SheetDaysLeft, _ = strconv.Atoi(sheetDaysLeftStr)
	}
	if len(row) > 4 {
		sheetReqDailyStr := fmt.Sprintf("%v", row[4])
		data.SheetReqDaily, _ = strconv.ParseFloat(sheetReqDailyStr, 64)
	}
	log.Printf("Дані з аркуша 'Звіт' успішно розпарсені: %+v", data)
	return data, nil
}

// AddGoalToSheet додає нову ціль на аркуш "МоїЦілі"
func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, chatID int64, goalData FinancialGoalData) error {
	sheetName := "МоїЦілі"
	log.Printf("Додавання цілі на аркуш '%s' для ChatID %d: %+v", sheetName, chatID, goalData)

	// Готуємо рядок даних для запису
	// A: ChatID, B: Amount, C: Currency, D: Days, E: SetDate, F: Status, G: OriginalText, H: ClosedDate
	var row []interface{}
	row = append(row, chatID, goalData.Amount, goalData.Currency, goalData.Days,
		goalData.SetDate.Format("2006-01-02"), // Форматуємо дату як РРРР-ММ-ДД
		"Активна",                             // Нова ціль завжди активна
		goalData.OriginalText,
		"", // ClosedDate поки порожній
	)

	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{row},
	}

	// Визначаємо діапазон для додавання (просто додаємо в кінець)
	// Google Sheets автоматично знайде перший порожній рядок
	appendRange := fmt.Sprintf("%s!A:H", sheetName)

	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
		ValueInputOption("USER_ENTERED"). // Дозволяє Google Sheets інтерпретувати дані (наприклад, дати)
		InsertDataOption("INSERT_ROWS").
		Do()

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

	// 1. Прочитати дані з аркуша, щоб знайти потрібний рядок
	// Читаємо колонки ChatID (A) та Status (F)
	readRange := fmt.Sprintf("%s!A:F", sheetName) // Читаємо до колонки статусу
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Помилка читання даних з аркуша '%s' для оновлення статусу (ChatID %d): %v", sheetName, chatID, err)
		return fmt.Errorf("не вдалося прочитати дані для оновлення статусу: %w", err)
	}

	var targetRowIndex = -1 // Індекс рядка в таблиці (1-based)
	if len(resp.Values) > 1 { // Пропускаємо рядок заголовків (якщо він є і дані починаються з другого)
		for i, row := range resp.Values {
			if i == 0 { continue } // Пропускаємо заголовок, якщо він у першому рядку діапазону читання

			if len(row) >= 6 { // Переконуємося, що є дані для ChatID та Status
				rowChatIDStr := fmt.Sprintf("%v", row[0]) // Колонка A - ChatID
				rowStatusStr := fmt.Sprintf("%v", row[5]) // Колонка F - Status

				rowChatID, _ := strconv.ParseInt(rowChatIDStr, 10, 64)

				if rowChatID == chatID && rowStatusStr == "Активна" {
					targetRowIndex = i + 1 // Номер рядка в Google Sheets (1-based)
					// Якщо у користувача може бути кілька активних цілей, тут потрібно буде обрати останню
					// або додати більш складну логіку ідентифікації цілі.
					// Зараз оновлюємо першу знайдену активну ціль цього користувача.
					break 
				}
			}
		}
	}

	if targetRowIndex == -1 {
		log.Printf("Не знайдено активної цілі для ChatID %d на аркуші '%s' для оновлення статусу.", chatID, sheetName)
		return fmt.Errorf("не знайдено активної цілі для оновлення")
	}

	// 2. Оновити статус (колонка F) та дату закриття (колонка H) у знайденому рядку
	var valuesToUpdate [][]interface{}
	// Формуємо дані для оновлення: перше значення для колонки F, друге для G (порожнє, щоб не чіпати), третє для H
	rowData := []interface{}{newStatus, closedDate.Format("2006-01-02")} // Статус, ДатаЗакриття

	updateRange := fmt.Sprintf("%s!F%d:H%d", sheetName, targetRowIndex, targetRowIndex) // Оновлюємо F та H
	// Щоб оновити лише F та H, а G залишити, треба робити два окремі запити або використовувати batchUpdate,
	// або передавати значення для G. Для простоти зараз оновимо лише F та H, G може бути затерто, якщо F і H не суміжні.
	// Краще оновлювати кожну комірку окремо або весь діапазон з проміжними значеннями.
	// Для оновлення окремих, несуміжних комірок краще використовувати BatchUpdateSpreadsheetRequest.
	// Для простоти, оновимо діапазон F:H, припускаючи, що G (OriginalText) не змінюється при закритті.
	// Або, якщо H - це дата закриття, то оновлюємо F та H.
	// Поточні колонки: F=Status, G=OriginalText, H=ClosedDate.
	// Нам потрібно оновити F (Status) та H (ClosedDate).
	// ValueRange для оновлення має бути для діапазону F<row>:H<row>
	// і містити значення для F, G, H. Якщо G не змінюємо, треба його прочитати і вставити.
	// ПРОСТІШИЙ ВАРІАНТ: оновити лише статус F і дату H окремими запитами або одним запитом, якщо вони суміжні
	// Якщо F - статус, H - дата закриття. Тоді G - OriginalText.
	// Оновлюємо F<targetRowIndex> та H<targetRowIndex>
	
	// Оновлюємо статус
	statusUpdateRange := fmt.Sprintf("%s!F%d", sheetName, targetRowIndex)
	statusValueRange := &sheets.ValueRange{ Values: [][]interface{}{{newStatus}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення статусу цілі на аркуші '%s' для ChatID %d (рядок %d): %v", sheetName, chatID, targetRowIndex, err)
		return fmt.Errorf("не вдалося оновити статус цілі: %w", err)
	}

	// Оновлюємо дату закриття
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


// getMotivation (залишається, але не використовується в GenerateProgressReport зараз)
// ... (код getMotivation залишається тут) ...
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
