package sheets

import (
	"fmt"

	"google.golang.org/api/sheets/v4"
)

func GenerateProgressReport(srv *sheets.Service, spreadsheetID string) (string, error) {
	readRange := "Arbitrage!A2:B"
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		return "", fmt.Errorf("не вдалося отримати дані з Google Sheets: %v", err)
	}

	if len(resp.Values) == 0 {
		return "Немає даних для звіту.", nil
	}

	report := "📊 *Прогрес за сьогодні:*\n"
	for _, row := range resp.Values {
		if len(row) < 2 {
			continue
		}
		report += fmt.Sprintf("▫️ *%s*: %s\n", row[0], row[1])
	}

	return report, nil
}
