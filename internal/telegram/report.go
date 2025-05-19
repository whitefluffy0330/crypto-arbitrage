package telegram

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

// monthNameUkrainian повертає українську назву місяця у родовому відмінку
func monthNameUkrainian(m time.Month) string {
	switch m {
	case time.January: return "Січня"; case time.February: return "Лютого"; case time.March: return "Березня"
	case time.April: return "Квітня"; case time.May: return "Травня"; case time.June: return "Червня"
	case time.July: return "Липня"; case time.August: return "Серпня"; case time.September: return "Вересня"
	case time.October: return "Жовтня"; case time.November: return "Листопада"; case time.December: return "Грудня"
	default: return ""
	}
}

func ReportProgress(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, cfg *config.Config) { // Змінено тип srv та cfg
	chatID := msg.Chat.ID
	var reportText string
	currentUserGoal, goalExists := GetUserGoal(chatID, srv, *cfg) // Використовує GetUserGoal з telegram.go, передаємо *cfg

	if !goalExists {
		reportText = "🎯 Спочатку встановіть фінансову ціль (/goal)."
	} else {
		reportSheetRange := fmt.Sprintf("%s!%s", cfg.SheetNameReport, cfg.SheetRangeReport)
		sheetData, errSheet := sheets.GenerateProgressReport(srv, cfg.SpreadsheetID, reportSheetRange)
		if errSheet != nil {
			goalMonth := currentUserGoal.SetDate.In(KyivLocation).Month() 
			goalYear := currentUserGoal.SetDate.In(KyivLocation).Year()
			reportText = fmt.Sprintf("📊 **Звіт** 📊\n🎯 **Ціль на %s %d:** `%.2f %s`\n⚠️ Помилка з Sheets ('%s'):\n`%v`\n🔥 %s",
				monthNameUkrainian(goalMonth), goalYear, currentUserGoal.Amount, currentUserGoal.Currency,
				cfg.SheetNameReport, errSheet, motivation.GetRandomMotivation())
		} else {
			// ... (решта вашої логіки ReportProgress з відповіді #68, переконайтеся, що KyivLocation використовується з telegram.KyivLocation)
			// Наприклад: goalTimeInKyiv := currentUserGoal.SetDate.In(KyivLocation)
			//           today := time.Now().In(KyivLocation)
			//           actualWorkingDaysLeftInMonth, _ := sheets.CountWorkingDaysInRange(srv, cfg.SpreadsheetID, cfg.SheetNameWorkLog, currentDateForCalc, endOfMonth)
			//           ... і так далі ...
			// Нижче приклад, як це може виглядати (потрібно адаптувати до вашої повної логіки)
			amountAchievedSoFar := sheetData.Income
			remainingToAchieve := currentUserGoal.Amount - amountAchievedSoFar
			var progressPercentage float64
			if currentUserGoal.Amount > 0 { progressPercentage = (amountAchievedSoFar / currentUserGoal.Amount) * 100 }
			reportText = fmt.Sprintf("📊 Звіт: Ціль %.2f %s. Досягнуто: %.2f %s (%.2f%%).",
				currentUserGoal.Amount, currentUserGoal.Currency,
				amountAchievedSoFar, currentUserGoal.Currency, progressPercentage) // Спрощений звіт
		}
	}
	response := tgbotapi.NewMessage(chatID, reportText)
	response.ParseMode = tgbotapi.ModeMarkdown
	if _, err := bot.Send(response); err != nil {
		log.Printf("Помилка надсилання звіту для чату %d: %v", chatID, err)
	}
}
