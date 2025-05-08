package telegram

import (
	"fmt"
	"log"
	// "math" // Не використовується
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

	currentUserGoal, goalExists := GetUserGoal(chatID, srv, cfg)

	if !goalExists {
		reportText = "🎯 Спочатку вам потрібно встановити фінансову ціль на поточний місяць за допомогою команди /goal."
		log.Printf("Запит звіту для ChatID %d, але ціль не встановлена.", chatID)
	} else {
		log.Printf("Генерація звіту для ChatID %d з ціллю: %+v", chatID, currentUserGoal)

		reportSheetRange := fmt.Sprintf("%s!%s", cfg.SheetNameReport, cfg.SheetRangeReport)
		sheetData, errSheet := sheets.GenerateProgressReport(srv, cfg.SpreadsheetID, reportSheetRange)

		if errSheet != nil {
			log.Printf("Помилка отримання даних з '%s': %v", cfg.SheetNameReport, errSheet)
			// Показуємо ціль, але повідомляємо про помилку читання даних для розрахунку
			goalMonth := currentUserGoal.SetDate.In(sheets.KyivLocation).Month()
			goalYear := currentUserGoal.SetDate.In(sheets.KyivLocation).Year()
			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Ваша Ціль на %s %d:** `%.2f %s`\n\n"+
					"⚠️ Не вдалося отримати актуальні дані з Google Sheets ('%s') для розрахунку прогресу:\n`%v`\n\n"+
					"🔥 %s",
				monthNameUkrainian(goalMonth), goalYear, // Назва місяця та рік
				currentUserGoal.Amount, currentUserGoal.Currency,
				cfg.SheetNameReport,
				errSheet,
				motivation.GetRandomMotivation(),
			)
		} else {
			// --- Розрахунки для місячної цілі ---
			amountAchievedSoFar := sheetData.Income
			remainingToAchieve := currentUserGoal.Amount - amountAchievedSoFar

			// Визначаємо початок і кінець місяця, до якого відноситься ціль
			goalTimeInKyiv := currentUserGoal.SetDate.In(sheets.KyivLocation)
			startOfMonth := time.Date(goalTimeInKyiv.Year(), goalTimeInKyiv.Month(), 1, 0, 0, 0, 0, sheets.KyivLocation)
			endOfMonth := startOfMonth.AddDate(0, 1, -1) // Останній день поточного місяця цілі

			// Визначаємо сьогоднішній день
			today := time.Now().In(sheets.KyivLocation)
			currentDateForCalc := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, sheets.KyivLocation)

			// Розрахунок календарних днів
			totalDaysInMonth := endOfMonth.Day()
			// Минуло днів у поточному місяці (включно з сьогодні)
			daysPassedInMonth := currentDateForCalc.Day() 
			// Залишилося календарних днів у місяці (включно з сьогодні)
			calendarDaysLeftInMonth := totalDaysInMonth - daysPassedInMonth + 1 
			if currentDateForCalc.After(endOfMonth) { // Якщо місяць цілі вже минув
				calendarDaysLeftInMonth = 0
				daysPassedInMonth = totalDaysInMonth // Вважаємо, що всі дні місяця минули
			}
			
			var progressPercentage float64
			if currentUserGoal.Amount > 0 {
				progressPercentage = (amountAchievedSoFar / currentUserGoal.Amount) * 100
				if progressPercentage > 100 { progressPercentage = 100 }
				if progressPercentage < 0 { progressPercentage = 0 }
			}

			// Розрахунок РОБОЧИХ днів, що залишилися до кінця місяця
			var actualWorkingDaysLeftInMonth int
			var errWorkingDays error
			// Рахуємо від сьогодні до кінця місяця
			if !currentDateForCalc.After(endOfMonth) { 
				actualWorkingDaysLeftInMonth, errWorkingDays = sheets.CountWorkingDaysInRange(srv, cfg.SpreadsheetID, cfg.SheetNameWorkLog, currentDateForCalc, endOfMonth)
				if errWorkingDays != nil {
					log.Printf("Помилка підрахунку робочих днів для ChatID %d: %v. Використовуються календарні дні.", chatID, errWorkingDays)
					actualWorkingDaysLeftInMonth = calendarDaysLeftInMonth // Відкат до календарних
				}
			} else {
				actualWorkingDaysLeftInMonth = 0 // Місяць цілі вже закінчився
			}

			// Розрахунок необхідного щоденного заробітку
			var requiredDailySmartStr string
			if remainingToAchieve <= 0 {
				requiredDailySmartStr = "0.00 (Ціль досягнуто!)"
			} else if actualWorkingDaysLeftInMonth <= 0 {
				requiredDailySmartStr = "∞ (Робочий час у місяці вийшов!)"
			} else {
				requiredDailySmart := remainingToAchieve / float64(actualWorkingDaysLeftInMonth)
				requiredDailySmartStr = fmt.Sprintf("%.2f", requiredDailySmart)
			}

			// Формуємо звіт
			goalMonth := startOfMonth.Month() // Місяць цілі
			goalYear := startOfMonth.Year()   // Рік цілі
			
			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Ціль на %s %d:**\n"+ // Місяць та рік
					"   Сума: `%.2f %s`\n\n"+
					"📈 **Поточний Прогрес (з '%s' на `%s`):\n"+
					"   Досягнуто (дохід): `%.2f %s`\n"+
					"   Залишилося досягти: `%.2f %s`\n"+
					"   Виконано: `%.2f %%`\n\n"+
					"⏳ **Час (до кінця %s):**\n"+ // Родовий відмінок місяця
					"   Минуло днів місяця: `%d` / `%d`\n"+
					"   Залишилося к.д.: `%d`\n"+
					"   Залишилося роб. д.: `%d`\n\n"+
					"💰 **Потрібно зараз (з урахуванням робочих днів):**\n"+
					"   Заробляти щодня: `%s %s`\n\n"+
					"🔥 %s",
				monthNameUkrainian(goalMonth), goalYear, // Назва місяця та рік
				currentUserGoal.Amount, currentUserGoal.Currency,

				cfg.SheetNameReport, sheetData.Date, // Назва аркуша звіту та дата з нього
				
				amountAchievedSoFar, currentUserGoal.Currency,
				remainingToAchieve, currentUserGoal.Currency,
				progressPercentage,
				
				monthNameUkrainian(goalMonth), // Назва місяця у родовому відмінку
				daysPassedInMonth, totalDaysInMonth, // Минуло / Всього днів у місяці
				calendarDaysLeftInMonth, // Залишилося календарних днів
				actualWorkingDaysLeftInMonth, // Залишилося робочих днів
				
				requiredDailySmartStr, currentUserGoal.Currency, // Необхідно щодня (розумний)
				
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
