package telegram

import (
	"fmt"
	"log"
	// "math" // ВИДАЛЕНО
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

// ReportProgress приймає cfg config.Config
func ReportProgress(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, cfg config.Config) { // <<< Сигнатура з cfg
	chatID := msg.Chat.ID
	var reportText string

	currentUserGoal, goalExists := GetUserGoal(chatID, srv, cfg) // Передаємо cfg

	if !goalExists {
		reportText = "🎯 Спочатку встановіть фінансову ціль..."
		log.Printf("Запит звіту для ChatID %d, але ціль не встановлена.", chatID)
	} else {
		log.Printf("Генерація звіту для ChatID %d з ціллю: %+v", chatID, currentUserGoal)

		reportSheetRange := fmt.Sprintf("%s!%s", cfg.SheetNameReport, cfg.SheetRangeReport)
		sheetData, errSheet := sheets.GenerateProgressReport(srv, cfg.SpreadsheetID, reportSheetRange) // Передаємо параметри з cfg

		if errSheet != nil {
			log.Printf("Помилка отримання даних з '%s': %v", cfg.SheetNameReport, errSheet)
			reportText = fmt.Sprintf( /* ... повідомлення про помилку ... */ )
		} else {
			// Розрахунки
			amountAchievedSoFar := sheetData.Income
			remainingToAchieve := currentUserGoal.Amount - amountAchievedSoFar
			
			var daysPassedSinceGoalSet int // Оголошено тут
			if !currentUserGoal.SetDate.IsZero() {
				todayInKyiv := time.Now().In(sheets.KyivLocation) // Використовуємо sheets.KyivLocation
				startDateOfGoalInKyiv := time.Date(currentUserGoal.SetDate.In(sheets.KyivLocation).Year(), currentUserGoal.SetDate.In(sheets.KyivLocation).Month(), currentUserGoal.SetDate.In(sheets.KyivLocation).Day(), 0, 0, 0, 0, sheets.KyivLocation)
				currentDateForCalc := time.Date(todayInKyiv.Year(), todayInKyiv.Month(), todayInKyiv.Day(), 0, 0, 0, 0, sheets.KyivLocation)
				daysPassedSinceGoalSet = int(currentDateForCalc.Sub(startDateOfGoalInKyiv).Hours() / 24)
				if daysPassedSinceGoalSet < 0 { daysPassedSinceGoalSet = 0 }
			} else {
				daysPassedSinceGoalSet = 0
			}

			calendarDaysLeftForGoal := currentUserGoal.Days - daysPassedSinceGoalSet // Оголошено тут
			if calendarDaysLeftForGoal < 0 { calendarDaysLeftForGoal = 0 }

			var progressPercentage float64 // Оголошено тут
			if currentUserGoal.Amount > 0 {
				progressPercentage = (amountAchievedSoFar / currentUserGoal.Amount) * 100
				if progressPercentage > 100 { progressPercentage = 100 }
				if progressPercentage < 0 { progressPercentage = 0 }
			}
			
			startDateOfGoalInKyiv := time.Date(currentUserGoal.SetDate.In(sheets.KyivLocation).Year(), currentUserGoal.SetDate.In(sheets.KyivLocation).Month(), currentUserGoal.SetDate.In(sheets.KyivLocation).Day(), 0, 0, 0, 0, sheets.KyivLocation)
			goalActualLastDay := startDateOfGoalInKyiv.AddDate(0, 0, currentUserGoal.Days-1)
			currentDateForWorkingDaysCalc := time.Now().In(sheets.KyivLocation)
			
			var actualWorkingDaysLeft int; var errWorkingDays error
			if !currentDateForWorkingDaysCalc.After(goalActualLastDay) {
				actualWorkingDaysLeft, errWorkingDays = sheets.CountWorkingDaysInRange(srv, cfg.SpreadsheetID, cfg.SheetNameWorkLog, currentDateForWorkingDaysCalc, goalActualLastDay) // Передаємо параметри з cfg
				if errWorkingDays != nil { log.Printf("Помилка підрахунку робочих днів: %v.", errWorkingDays); actualWorkingDaysLeft = calendarDaysLeftForGoal }
			} else { actualWorkingDaysLeft = 0; log.Printf("Ціль вже мала завершитися (%s).", goalActualLastDay.Format("02.01.2006")) }

			var requiredDailySmartStr string
			if remainingToAchieve <= 0 { requiredDailySmartStr = "0.00 (Ціль досягнуто!)" } else if actualWorkingDaysLeft <= 0 { requiredDailySmartStr = "∞ (Час вийшов!)" } else { requiredDailySmart := remainingToAchieve / float64(actualWorkingDaysLeft); requiredDailySmartStr = fmt.Sprintf("%.2f", requiredDailySmart) }
			
			// Формування звіту з ВИКОРИСТАННЯМ оголошених змінних
			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Встановлена Ціль:**\n"+
					"   Сума: `%.2f %s`\n"+
					"   Загальний термін: `%d к.д.`\n"+
					"   Встановлено: `%s` (до `%s`)\n\n"+
					"📈 **Поточний Прогрес (з '%s' на `%s`):\n"+
					"   Досягнуто: `%.2f %s`\n"+
					"   Залишилося: `%.2f %s`\n"+
					"   Виконано: `%.2f %%`\n\n"+ // Використовуємо progressPercentage
					"⏳ **Час:**\n"+
					"   Минуло к.д.: `%d` / `%d`\n"+ // Використовуємо daysPassedSinceGoalSet
					"   Залишилося к.д.: `%d`\n"+ // Використовуємо calendarDaysLeftForGoal
					"   Залишилося роб. д.: `%d`\n\n"+ 
					"💰 **Потрібно зараз:**\n"+
					"   Заробляти щодня (роб.): `%s %s`\n\n"+
					"🔥 %s",
				currentUserGoal.Amount, currentUserGoal.Currency, currentUserGoal.Days,
				currentUserGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006"), goalActualLastDay.Format("02.01.2006"),
				cfg.SheetNameReport, sheetData.Date,
				amountAchievedSoFar, currentUserGoal.Currency,
				remainingToAchieve, currentUserGoal.Currency,
				progressPercentage, // <<< АРГУМЕНТ Є
				daysPassedSinceGoalSet, currentUserGoal.Days, // <<< АРГУМЕНТ Є
				calendarDaysLeftForGoal, // <<< АРГУМЕНТ Є
				actualWorkingDaysLeft, 
				requiredDailySmartStr, currentUserGoal.Currency,
				motivation.GetRandomMotivation(),
			)
		}
	}

	response := tgbotapi.NewMessage(chatID, reportText); response.ParseMode = tgbotapi.ModeMarkdown
	if _, err := bot.Send(response); err != nil { log.Printf("Помилка надсилання звіту: %v", err) }
}
