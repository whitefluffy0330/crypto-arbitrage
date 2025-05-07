package telegram

import (
	"fmt"
	"log"
	"math"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Додаємо імпорт config
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

// ReportProgress тепер приймає cfg config.Config
func ReportProgress(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, cfg config.Config) {
	chatID := msg.Chat.ID
	var reportText string

	// Передаємо cfg в GetUserGoal
	currentUserGoal, goalExists := GetUserGoal(chatID, srv, cfg) // <<< ЗМІНЕНО ТУТ

	if !goalExists {
		reportText = "🎯 Спочатку вам потрібно встановити фінансову ціль за допомогою команди /goal або кнопки \"🎯 Моя ціль\"."
		log.Printf("Запит звіту для ChatID %d, але ціль не встановлена.", chatID)
	} else {
		log.Printf("Генерація звіту для ChatID %d з ціллю: %+v", chatID, currentUserGoal)

		// Формуємо повний діапазон для звіту з конфігурації
		reportSheetRange := fmt.Sprintf("%s!%s", cfg.SheetNameReport, cfg.SheetRangeReport)
		// Викликаємо GenerateProgressReport з параметрами з cfg
		sheetData, errSheet := sheets.GenerateProgressReport(srv, cfg.SpreadsheetID, reportSheetRange) // <<< ЗМІНЕНО ТУТ
		if errSheet != nil {
			log.Printf("Помилка отримання даних з Google Sheets (аркуш '%s') для звіту ChatID %d: %v", cfg.SheetNameReport, chatID, errSheet)
			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Ваша Ціль:** `%.2f %s` за `%d днів` (встановлено %s).\n\n"+
					"⚠️ Не вдалося отримати актуальні дані з Google Sheets ('%s') для розрахунку прогресу:\n`%v`\n\n"+
					"🔥 %s",
				currentUserGoal.Amount, currentUserGoal.Currency, currentUserGoal.Days,
				currentUserGoal.SetDate.In(kyivLocation).Format("02.01.2006"),
				cfg.SheetNameReport, // Додано назву аркуша в повідомлення про помилку
				errSheet,
				motivation.GetRandomMotivation(),
			)
		} else {
			amountAchievedSoFar := sheetData.Income
			remainingToAchieve := currentUserGoal.Amount - amountAchievedSoFar

			var daysPassedSinceGoalSet int
			if !currentUserGoal.SetDate.IsZero() {
				todayInKyiv := time.Now().In(kyivLocation)
				startDateOfGoalInKyiv := time.Date(currentUserGoal.SetDate.In(kyivLocation).Year(), currentUserGoal.SetDate.In(kyivLocation).Month(), currentUserGoal.SetDate.In(kyivLocation).Day(), 0, 0, 0, 0, kyivLocation)
				currentDateForCalc := time.Date(todayInKyiv.Year(), todayInKyiv.Month(), todayInKyiv.Day(), 0, 0, 0, 0, kyivLocation)
				daysPassedSinceGoalSet = int(currentDateForCalc.Sub(startDateOfGoalInKyiv).Hours() / 24)
				if daysPassedSinceGoalSet < 0 { daysPassedSinceGoalSet = 0 }
			} else {
				daysPassedSinceGoalSet = 0
			}

			calendarDaysLeftForGoal := currentUserGoal.Days - daysPassedSinceGoalSet
			if calendarDaysLeftForGoal < 0 {
				calendarDaysLeftForGoal = 0
			}

			var progressPercentage float64
			if currentUserGoal.Amount > 0 {
				progressPercentage = (amountAchievedSoFar / currentUserGoal.Amount) * 100
				if progressPercentage > 100 { progressPercentage = 100 }
				if progressPercentage < 0 { progressPercentage = 0 }
			}

			startDateOfGoalInKyiv := time.Date(currentUserGoal.SetDate.In(kyivLocation).Year(), currentUserGoal.SetDate.In(kyivLocation).Month(), currentUserGoal.SetDate.In(kyivLocation).Day(), 0, 0, 0, 0, kyivLocation)
			goalActualLastDay := startDateOfGoalInKyiv.AddDate(0, 0, currentUserGoal.Days-1)
			currentDateForWorkingDaysCalc := time.Now().In(kyivLocation)

			var actualWorkingDaysLeft int
			var errWorkingDays error

			if !currentDateForWorkingDaysCalc.After(goalActualLastDay) {
				// Викликаємо CountWorkingDaysInRange з параметрами з cfg
				actualWorkingDaysLeft, errWorkingDays = sheets.CountWorkingDaysInRange(srv, cfg.SpreadsheetID, cfg.SheetNameWorkLog, currentDateForWorkingDaysCalc, goalActualLastDay) // <<< ЗМІНЕНО ТУТ
				if errWorkingDays != nil {
					log.Printf("Помилка підрахунку робочих днів для ChatID %d: %v. Будуть використані календарні дні.", chatID, errWorkingDays)
					actualWorkingDaysLeft = calendarDaysLeftForGoal
				}
			} else {
				actualWorkingDaysLeft = 0
				log.Printf("Підрахунок робочих днів: ціль вже мала завершитися (%s), робочих днів 0.", goalActualLastDay.Format("2006-01-02"))
			}

			var requiredDailySmartStr string
			if remainingToAchieve <= 0 {
				requiredDailySmartStr = "0.00 (Ціль досягнуто!)"
			} else if actualWorkingDaysLeft <= 0 {
				requiredDailySmartStr = "∞ (Робочий час вийшов або не заплановано робочих днів!)"
			} else {
				requiredDailySmart := remainingToAchieve / float64(actualWorkingDaysLeft)
				requiredDailySmartStr = fmt.Sprintf("%.2f", requiredDailySmart)
			}

			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Встановлена Ціль:**\n"+
					"   Сума: `%.2f %s`\n"+
					"   Загальний термін: `%d календарних днів`\n"+
					"   Встановлено: `%s` (завершується до кінця дня %s)\n\n"+
					"📈 **Поточний Прогрес (з аркуша '%s' станом на `%s`):\n"+ // Використовуємо cfg.SheetNameReport
					"   Досягнуто (дохід): `%.2f %s`\n"+
					"   Залишилося досягти: `%.2f %s`\n"+
					"   Виконано: `%.2f %%`\n\n"+
					"⏳ **Час:**\n"+
					"   Календарних днів минуло: `%d` / `%d`\n"+
					"   Календарних днів залишилося до кінця цілі: `%d`\n"+
					"   Заплановано робочих днів до кінця цілі: `%d`\n\n"+
					"💰 **Потрібно зараз (з урахуванням робочих днів):**\n"+
					"   Заробляти щодня: `%s %s`\n\n"+
					"🔥 %s",
				currentUserGoal.Amount, currentUserGoal.Currency,
				currentUserGoal.Days,
				currentUserGoal.SetDate.In(kyivLocation).Format("02.01.2006"),
				goalActualLastDay.Format("02.01.2006"),
				cfg.SheetNameReport, // Використовуємо назву аркуша з конфігурації
				sheetData.Date,
				amountAchievedSoFar, currentUserGoal.Currency,
				remainingToAchieve, currentUserGoal.Currency,
				progressPercentage,
				daysPassedSinceGoalSet, currentUserGoal.Days,
				calendarDaysLeftForGoal,
				actualWorkingDaysLeft,
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
