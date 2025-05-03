package sheets

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/api/sheets/v4"
)

func GenerateProgressReport(srv *sheets.Service, spreadsheetID string) string {
	// Тут приклад простої реалізації
	readRange := "A1:B10"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		return fmt.Sprintf("Помилка отримання звіту: %v", err)
	}

	report := "📊 Прогрес:\n"
	for _, row := range resp.Values {
		if len(row) >= 2 {
			report += fmt.Sprintf("- %s: %s\n", row[0], row[1])
		}
	}
	return report
}
