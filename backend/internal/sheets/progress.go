package sheets

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Service — обгортка над Google Sheets API
type Service struct {
	srv           *sheets.Service
	spreadsheetID string
}

// InitGoogleSheets — створює сервіс Sheets API
func InitGoogleSheets() *Service {
	ctx := context.Background()

	sheetsService, err := sheets.NewService(ctx, option.WithCredentialsFile("internal/credentials.json"))
	if err != nil {
		log.Fatalf("Unable to create Sheets service: %v", err)
	}

	// замінимо на правильний ID з .env
	return &Service{
		srv: sheetsService,
	}
}

// SetSpreadsheetID — встановити ID таблиці після ініціалізації
func (s *Service) SetSpreadsheetID(id string) {
	s.spreadsheetID = id
}

// GenerateProgressReport — просто повертає зведення (буде оновлюватись)
func (s *Service) GenerateProgressReport() string {
	// Пізніше замінимо на реальні обчислення
	return fmt.Sprintf("🗓️ %s\nПрогрес: 6%%\nВсе йде за планом!", time.Now().Format("02.01.2006"))
}
