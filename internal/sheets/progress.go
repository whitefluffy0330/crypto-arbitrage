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
	Srv           *sheets.Service
	SpreadsheetID string
}

// InitGoogleSheets — створює сервіс Sheets API
func InitGoogleSheets() *Service {
	ctx := context.Background()

	sheetsService, err := sheets.NewService(ctx, option.WithCredentialsFile("internal/credentials.json"))
	if err != nil {
		log.Fatalf("Unable to create Sheets service: %v", err)
	}

	return &Service{
		Srv: sheetsService,
	}
}

// SetSpreadsheetID — встановити ID таблиці після ініціалізації
func (s *Service) SetSpreadsheetID(id string) {
	s.SpreadsheetID = id
}

// GenerateProgressReport — просто повертає зведення (буде оновлюватись)
func (s *Service) GenerateProgressReport() string {
	return fmt.Sprintf("📊 Звіт за день:\n\nПрогрес: 64%%\nВсе йде за планом!\n\n%s", time.Now().Format("02.01.2006"))
}
