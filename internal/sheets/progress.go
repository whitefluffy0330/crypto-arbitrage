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

func NewService(credentialsJSON []byte) (*sheets.Service, error) {
	ctx := context.Background()
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(credentialsJSON))
	if err != nil {
		return nil, fmt.Errorf("не вдалося створити клієнт Sheets: %w", err)
	}
	return srv, nil
}

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
		errMsg := fmt.Sprintf("недостатньо даних у діапазоні %s: отримано %d рядків (або недостатньо колонок), очікувався 1 рядок з 5 колонками", readRange, len(resp.Values))
		log.Println(errMsg)
		return data, fmt.Errorf(errMsg)
	}

	row := resp.Values[0]
	if len(row) > 0 { data.Date = fmt.Sprintf("%v", row[0]) }
	if len(row) > 1 {
		incomeStr := fmt.Sprintf("%v", row[1])
		data.Income, err = strconv.ParseFloat(incomeStr, 64)
		if err != nil {
			log.Printf("Помилка парсингу доходу '%s': %v", incomeStr, err)
			// Повертаємо помилку, якщо критично, або встановлюємо значення за замовчуванням/продовжуємо
		}
	}
	if len(row) > 2 {
		sheetGoalStr := fmt.Sprintf("%v", row[2])
		data.SheetGoal, err = strconv.ParseFloat(sheetGoalStr, 64)
		if err != nil {
			log.Printf("Помилка парсингу мети з таблиці '%s': %v", sheetGoalStr, err)
		}
	}
	if len(row) > 3 {
		sheetDaysLeftStr := fmt.Sprintf("%v", row[3])
		data.SheetDaysLeft, err = strconv.Atoi(sheetDaysLeftStr)
		if err != nil {
			log.Printf("Помилка парсингу 'днів залишилося' з таблиці '%s': %v", sheetDaysLeftStr, err)
		}
	}
	if len(row) > 4 {
		sheetReqDailyStr := fmt.Sprintf("%v", row[4])
		data.SheetReqDaily, err = strconv.ParseFloat(sheetReqDailyStr, 64)
		if err != nil {
			log.Printf("Помилка парсингу 'потрібно щодня' з таблиці '%s': %v", sheetReqDailyStr, err)
		}
	}
	log.Printf("Дані з аркуша 'Звіт' успішно розпарсені: %+v", data)
	return data, nil
}

func AddGoalToSheet(srv *sheets.Service, spreadsheetID string, chatID int64, goalData FinancialGoalData) error {
	sheetName := "МоїЦілі"
	log.Printf("Додавання цілі на аркуш '%s' для ChatID %d: %+v", sheetName, chatID, goalData)

	var row []interface{}
	row = append(row, chatID, goalData.Amount, goalData.Currency, goalData.Days,
		goalData.SetDate.Format("2006-01-02"),
		"Активна",
		goalData.OriginalText,
		"", 
	)

	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{row},
	}

	appendRange := fmt.Sprintf("%s!A:H", sheetName)

	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, appendRange, valueRange).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").
		Do()

	if err != nil {
		log.Printf("Помилка запису цілі на аркуш '%s' для ChatID %d: %v", sheetName, chatID, err)
		return fmt.Errorf("не вдалося записати ціль у таблицю: %w", err)
	}

	log.Printf("Ціль для ChatID %d успішно записана на аркуш '%s'", chatID, sheetName)
	return nil
}

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
		// Починаємо з 1, оскільки Values[0] може бути заголовком, або якщо дані гарантовано з 2-го рядка
		// Якщо ваш аркуш "МоїЦілі" має заголовок у 1-му рядку, і дані починаються з 2-го, то ітерацію треба починати з resp.Values[1]
		// Або, якщо Get читає весь аркуш, то targetRowIndex = i + 1 (якщо Values[0] це рядок 1). 
		// Якщо Get читає, наприклад, з A2:F, то індекс i вже правильний для відносного рядка, а для абсолютного треба додати 2.
		// Для простоти припустимо, що resp.Values[0] - це перший рядок ДАНИХ (не заголовків) або заголовки вже відфільтровані.
		// Якщо заголовки є і читаються, цикл for i, row := range resp.Values, if i == 0 {continue}
		for i, row := range resp.Values {
			// Якщо у вас є рядок заголовків, який потрапляє в resp.Values, його треба пропустити
			// Наприклад, якщо ви знаєте, що перший рядок - це заголовки:
			// if i == 0 && (row[0] == "ChatID" || row[0] == "ID чату користувача") { // Приклад перевірки заголовка
			// 	continue
			// }

			if len(row) >= 6 { 
				rowChatIDStr := fmt.Sprintf("%v", row[0]) 
				rowStatusStr := fmt.Sprintf("%v", row[5]) 
				rowChatID, _ := strconv.ParseInt(rowChatIDStr, 10, 64)

				if rowChatID == chatID && rowStatusStr == "Активна" {
					// targetRowIndex тут буде 0-based індексом у зрізі resp.Values.
					// Для формування діапазону Google Sheets нам потрібен 1-based номер рядка.
					// Якщо resp.Values[0] - це рядок 1 на аркуші, то номер рядка = i + 1.
					// Якщо resp.Values[0] - це рядок 2 на аркуші (бо A1 - заголовок), то номер рядка = i + 2.
					// Припускаємо, що ви читаєте ВЕСЬ аркуш або з A1, і A1 - це заголовок.
					// Отже, якщо дані починаються з рядка 2, то індекс рядка в таблиці буде i + 1 (якщо i - індекс в resp.Values, що не включає заголовок)
					// Або, якщо i - індекс в resp.Values, що ВКЛЮЧАЄ заголовок, то i + 1.
					// Давайте припустимо, що resp.Values - це всі рядки, включаючи заголовок у Values[0]
					if i == 0 { continue } // Пропускаємо заголовок, якщо він є в прочитаному діапазоні
					targetRowIndex = i + 1 // 1-based індекс рядка на аркуші
					break 
				}
			}
		}
	}

	if targetRowIndex == -1 {
		log.Printf("Не знайдено активної цілі для ChatID %d на аркуші '%s' для оновлення статусу.", chatID, sheetName)
		return fmt.Errorf("не знайдено активної цілі для оновлення")
	}

	// Видалено невикористані змінні: valuesToUpdate, rowData, updateRange

	statusUpdateRange := fmt.Sprintf("%s!F%d", sheetName, targetRowIndex)
	statusValueRange := &sheets.ValueRange{ Values: [][]interface{}{{newStatus}} }
	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, statusUpdateRange, statusValueRange).
		ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка оновлення статусу цілі на аркуші '%s' для ChatID %d (рядок %d): %v", sheetName, chatID, targetRowIndex, err)
		return fmt.Errorf("не вдалося оновити статус цілі: %w", err)
	}

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
