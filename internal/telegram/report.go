package telegram

import (
	"fmt"
	"log"
	// "math" // ВИДАЛЕНО
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config" // Додаємо імпорт
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

// ReportProgress тепер приймає cfg config.Config
func ReportProgress(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, cfg config.Config) { // <<< ЗМІНЕНО СИГНАТУРУ
	chatID := msg.Chat.ID
	var reportText string

	currentUserGoal, goalExists := GetUserGoal(chatID, srv, cfg) // Передаємо cfg

	if !goalExists {
		reportText = "🎯 Спочатку вам потрібно встановити фінансову ціль..."
		log.Printf("Запит звіту для ChatID %d, але ціль не встановлена.", chatID)
	} else {
		log.Printf("Генерація звіту для ChatID %d з ціллю: %+v", chatID, currentUserGoal)

		reportSheetRange := fmt.Sprintf("%s!%s", cfg.SheetNameReport, cfg.SheetRangeReport)
		sheetData, errSheet := sheets.GenerateProgressReport(srv, cfg.SpreadsheetID, reportSheetRange) // Передаємо параметри з cfg

		if errSheet != nil {
			log.Printf("Помилка отримання даних з Google Sheets ('%s'): %v", cfg.SheetNameReport, errSheet)
			reportText = fmt.Sprintf( /* ... повідомлення про помилку ... */ )
		} else {
			amountAchievedSoFar := sheetData.Income
			remainingToAchieve := currentUserGoal.Amount - amountAchievedSoFar
			// ... решта розрахунків ...

			startDateOfGoalInKyiv := time.Date(currentUserGoal.SetDate.In(sheets.KyivLocation).Year(), currentUserGoal.SetDate.In(sheets.KyivLocation).Month(), currentUserGoal.SetDate.In(sheets.KyivLocation).Day(), 0, 0, 0, 0, sheets.KyivLocation) // Використовуємо sheets.KyivLocation
			goalActualLastDay := startDateOfGoalInKyiv.AddDate(0, 0, currentUserGoal.Days-1)
			currentDateForWorkingDaysCalc := time.Now().In(sheets.KyivLocation) // Використовуємо sheets.KyivLocation

			var actualWorkingDaysLeft int
			var errWorkingDays error

			if !currentDateForWorkingDaysCalc.After(goalActualLastDay) {
				actualWorkingDaysLeft, errWorkingDays = sheets.CountWorkingDaysInRange(srv, cfg.SpreadsheetID, cfg.SheetNameWorkLog, currentDateForWorkingDaysCalc, goalActualLastDay) // Передаємо параметри з cfg
				if errWorkingDays != nil {
					log.Printf("Помилка підрахунку робочих днів: %v.", errWorkingDays)
					// ... обробка помилки ...
				}
			} else {
				// ...
			}

			var requiredDailySmartStr string
			if remainingToAchieve <= 0 { requiredDailySmartStr = "0.00 (Ціль досягнуто!)" } else if actualWorkingDaysLeft <= 0 { requiredDailySmartStr = "∞ (Робочий час вийшов!)" } else { requiredDailySmart := remainingToAchieve / float64(actualWorkingDaysLeft); requiredDailySmartStr = fmt.Sprintf("%.2f", requiredDailySmart) }

			// ... решта коду форматування звіту ...
			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Встановлена Ціль:**\n"+
					"   Сума: `%.2f %s`\n"+
					"   Загальний термін: `%d к.д.`\n"+ // к.д. = календарних днів
					"   Встановлено: `%s` (до `%s`)\n\n"+
					"📈 **Поточний Прогрес (з '%s' на `%s`):\n"+
					"   Досягнуто: `%.2f %s`\n"+
					"   Залишилося: `%.2f %s`\n"+
					"   Виконано: `%.2f %%`\n\n"+
					"⏳ **Час:**\n"+
					"   Минуло к.д.: `%d` / `%d`\n"+
					"   Залишилося к.д.: `%d`\n"+
					"   Залишилося роб. д.: `%d`\n\n"+ // роб. д. = робочих днів
					"💰 **Потрібно зараз:**\n"+
					"   Заробляти щодня (роб.): `%s %s`\n\n"+
					"🔥 %s",
				currentUserGoal.Amount, currentUserGoal.Currency, currentUserGoal.Days,
				currentUserGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006"), goalActualLastDay.Format("02.01.2006"), // Використовуємо sheets.KyivLocation
				cfg.SheetNameReport, sheetData.Date,
				amountAchievedSoFar, currentUserGoal.Currency,
				remainingToAchieve, currentUserGoal.Currency, progressPercentage,
				daysPassedSinceGoalSet, currentUserGoal.Days, calendarDaysLeftForGoal,
				actualWorkingDaysLeft, requiredDailySmartStr, currentUserGoal.Currency,
				motivation.GetRandomMotivation(),
			)
		}
	}

	response := tgbotapi.NewMessage(chatID, reportText); response.ParseMode = tgbotapi.ModeMarkdown
	if _, err := bot.Send(response); err != nil { log.Printf("Помилка надсилання звіту: %v", err) }
}
