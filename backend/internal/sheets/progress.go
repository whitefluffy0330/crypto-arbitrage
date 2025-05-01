package sheets

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"google.golang.org/api/sheets/v4"
)

func UpdateGoalProgress(srv *sheets.Service, spreadsheetID string) {
	ctx := context.Background()

	// Зчитуємо список цілей
	goalsResp, err := srv.Spreadsheets.Values.Get(spreadsheetID, "Цілі!A2:E").Context(ctx).Do()
	if err != nil {
		log.Printf("Помилка при читанні 'Цілі': %v", err)
		return
	}

	for i, row := range goalsResp.Values {
		if len(row) < 5 {
			continue
		}
		status := fmt.Sprintf("%v", row[3])
		if status == "Активна" {
			planStr := fmt.Sprintf("%v", row[1])
			factStr := fmt.Sprintf("%v", row[2])

			plan, _ := strconv.ParseFloat(strings.ReplaceAll(planStr, ",", "."), 64)
			fact, _ := strconv.ParseFloat(strings.ReplaceAll(factStr, ",", "."), 64)

			if plan == 0 {
				continue
			}

			progress := (fact / plan) * 100
			progressStr := fmt.Sprintf("%.0f%%", progress)

			cell := fmt.Sprintf("E%d", i+2)
			_, err := srv.Spreadsheets.Values.Update(spreadsheetID, "Цілі!"+cell, &sheets.ValueRange{
				Values: [][]interface{}{{progressStr}},
			}).ValueInputOption("USER_ENTERED").Context(ctx).Do()
			if err != nil {
				log.Printf("Помилка при оновленні прогресу: %v", err)
			} else {
				log.Printf("Прогрес оновлено: %s", progressStr)
			}
			break
		}
	}
}
