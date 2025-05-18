package telegram // Залишаємо в тому ж пакеті telegram

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
func monthNameUkrainian(m time.Month) string { // Ця функція залишається тут
	switch m {
	case time.January: return "Січня"
	case time.February: return "Лютого"
	case time.March: return "Березня"
	case time.April: return "Квітня"
	case time.May: return "Травня"
	case time.June: return "Червня"
	case time.July: return "Липня"
	case time.August: return "Серпня"
	case time.September: return "Вересня"
	case time.October: return "Жовтня"
	case time.November: return "Листопада"
	case time.December: return "Грудня"
	default: return ""
	}
}

// ReportProgress генерує звіт на основі місячної цілі
func ReportProgress(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, cfg config.Config) {
	chatID := msg.Chat.ID
	var reportText string

	currentUserGoal, goalExists := GetUserGoal(chatID, srv, cfg) // Викликаємо GetUserGoal з telegram.go

	if !goalExists {
		reportText = "🎯 Спочатку вам потрібно встановити фінансову ціль на поточний місяць за допомогою команди /goal."
		log.Printf("Запит звіту для ChatID %d, але ціль не встановлена.", chatID)
	} else {
		log.Printf("Генерація звіту для ChatID %d з ціллю: %+v", chatID, currentUserGoal)
		reportSheetRange := fmt.Sprintf("%s!%s", cfg.SheetNameReport, cfg.SheetRangeReport)
		sheetData, errSheet := sheets.GenerateProgressReport(srv, cfg.SpreadsheetID, reportSheetRange)

		if errSheet != nil {
			log.Printf("Помилка отримання даних з '%s': %v", cfg.SheetNameReport, errSheet)
			goalMonth := currentUserGoal.SetDate.In(KyivLocation).Month() // Використовуємо KyivLocation з telegram.go
			goalYear := currentUserGoal.SetDate.In(KyivLocation).Year()
			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Ваша Ціль на %s %d:** `%.2f %s`\n\n"+
					"⚠️ Не вдалося отримати актуальні дані з Google Sheets ('%s') для розрахунку прогресу:\n`%v`\n\n"+
					"🔥 %s",
				monthNameUkrainian(goalMonth), goalYear, 
				currentUserGoal.Amount, currentUserGoal.Currency,
				cfg.SheetNameReport,
				errSheet,
				motivation.GetRandomMotivation(),
			)
		} else {
			amountAchievedSoFar := sheetData.Income
			remainingToAchieve := currentUserGoal.Amount - amountAchievedSoFar
			goalTimeInKyiv := currentUserGoal.SetDate.In(KyivLocation) // Використовуємо KyivLocation з telegram.go
			startOfMonth := time.Date(goalTimeInKyiv.Year(), goalTimeInKyiv.Month(), 1, 0, 0, 0, 0, KyivLocation)
			endOfMonth := startOfMonth.AddDate(0, 1, -1) 
			today := time.Now().In(KyivLocation)
			currentDateForCalc := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, KyivLocation)
			totalDaysInMonth := endOfMonth.Day()
			daysPassedInMonth := currentDateForCalc.Day() 
			calendarDaysLeftInMonth := totalDaysInMonth - daysPassedInMonth + 1 
			if currentDateForCalc.After(endOfMonth) { 
				calendarDaysLeftInMonth = 0
				daysPassedInMonth = totalDaysInMonth
			}
			var progressPercentage float64
			if currentUserGoal.Amount > 0 {
				progressPercentage = (amountAchievedSoFar / currentUserGoal.Amount) * 100
				if progressPercentage > 100 { progressPercentage = 100 }
				if progressPercentage < 0 { progressPercentage = 0 }
			}
			var actualWorkingDaysLeftInMonth int
			var errWorkingDays error
			if !currentDateForCalc.After(endOfMonth) { 
				actualWorkingDaysLeftInMonth, errWorkingDays = sheets.CountWorkingDaysInRange(srv, cfg.SpreadsheetID, cfg.SheetNameWorkLog, currentDateForCalc, endOfMonth)
				if errWorkingDays != nil {
					log.Printf("Помилка підрахунку робочих днів для ChatID %d: %v. Використовуються календарні дні.", chatID, errWorkingDays)
					actualWorkingDaysLeftInMonth = calendarDaysLeftInMonth
				}
			} else {
				actualWorkingDaysLeftInMonth = 0
			}
			var requiredDailySmartStr string
			if remainingToAchieve <= 0 {
				requiredDailySmartStr = "0.00 (Ціль досягнуто!)"
			} else if actualWorkingDaysLeftInMonth <= 0 {
				requiredDailySmartStr = "∞ (Робочий час у місяці вийшов!)"
			} else {
				requiredDailySmart := remainingToAchieve / float64(actualWorkingDaysLeftInMonth)
				requiredDailySmartStr = fmt.Sprintf("%.2f", requiredDailySmart)
			}
			goalMonth := startOfMonth.Month()
			goalYear := startOfMonth.Year()
			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Ціль на %s %d:**\n"+ 
					"   Сума: `%.2f %s`\n\n"+
					"📈 **Поточний Прогрес (з '%s' на `%s`):\n"+
					"   Досягнуто (дохід): `%.2f %s`\n"+
					"   Залишилося досягти: `%.2f %s`\n"+
					"   Виконано: `%.2f %%`\n\n"+
					"⏳ **Час (до кінця %s):**\n"+ 
					"   Минуло днів місяця: `%d` / `%d`\n"+
					"   Залишилося к.д.: `%d`\n"+
					"   Залишилося роб. д.: `%d`\n\n"+
					"💰 **Потрібно зараз (з урахуванням робочих днів):**\n"+
					"   Заробляти щодня: `%s %s`\n\n"+
					"🔥 %s",
				monthNameUkrainian(goalMonth), goalYear, 
				currentUserGoal.Amount, currentUserGoal.Currency,
				cfg.SheetNameReport, sheetData.Date, 
				amountAchievedSoFar, currentUserGoal.Currency,
				remainingToAchieve, currentUserGoal.Currency,
				progressPercentage,
				monthNameUkrainian(goalMonth), 
				daysPassedInMonth, totalDaysInMonth, 
				calendarDaysLeftInMonth, 
				actualWorkingDaysLeftInMonth, 
				requiredDailySmartStr, currentUserGoal.Currency, 
				motivation.GetRandomMotivation(),
			)
		}
	}
	response := tgbotapi.NewMessage(chatID, reportText)
	response.ParseMode = tgbotapi.ModeMarkdown
	if _, err := bot.Send(response); err != nil {
		log.Printf("Помилка надсилання звіту про прогрес для чату %d: %v", chatID, err)
	}
}
