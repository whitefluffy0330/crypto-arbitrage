package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

func main() {
	// Завантажуємо змінні середовища
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Помилка завантаження .env файлу")
	}

	spreadsheetID := os.Getenv("SPREADSHEET_ID")
	if spreadsheetID == "" {
		log.Fatal("SPREADSHEET_ID не встановлений у .env")
	}

	// Підключення до Google Sheets API
	credsData, err := os.ReadFile("internal/credentials.json")
	if err != nil {
		log.Fatalf("Не знайдено файл credentials.json: %v", err)
	}

	config, err := google.JWTConfigFromJSON(credsData, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Помилка створення конфігурації: %v", err)
	}

	client := config.Client(context.Background())

	srv, err := sheets.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Помилка створення сервісу Sheets API: %v", err)
	}

	// Створення нового листа "Мапа доходу"
	addSheetRequest := &sheets.AddSheetRequest{
		Properties: &sheets.SheetProperties{
			Title: "Мапа доходу",
		},
	}

	requests := []*sheets.Request{
		{
			AddSheet: addSheetRequest,
		},
	}

	batchUpdate := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	}

	_, err = srv.Spreadsheets.BatchUpdate(spreadsheetID, batchUpdate).Do()
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			log.Fatalf("Помилка створення листа: %v", err)
		}
	}

	// Заповнення заголовків
	values := [][]interface{}{
		{"Напрямок", "Статус", "Поточний прибуток ($)", "Цільовий прибуток ($)", "Прогрес (%)", "Дата старту", "План дій"},
	}

	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, "Мапа доходу!A1:G1", &sheets.ValueRange{
		Values: values,
	}).ValueInputOption("RAW").Do()
	if err != nil {
		log.Fatalf("Помилка запису заголовків: %v", err)
	}

	fmt.Println("✅ Мапу доходу створено успішно!")
}
