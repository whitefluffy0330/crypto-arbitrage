package telegram

import (
	"fmt"
	"log"
	"math"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

func ReportProgress(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, spreadsheetID string) {
	chatID := msg.Chat.ID
	var reportText string

	currentUserGoal, goalExists := GetUserGoal(chatID, srv, spreadsheetID) // Передаємо srv та spreadsheetID

	if !goalExists {
		reportText = "🎯 Спочатку вам потрібно встановити фінансову ціль за допомогою команди /goal або кнопки \"🎯 Моя ціль\"."
		log.Printf("Запит звіту для ChatID %d, але ціль не встановлена.", chatID)
	} else {
		log.Printf("Генерація звіту для ChatID %d з ціллю: %+v", chatID, currentUserGoal)

		sheetData, errSheet := sheets.GenerateProgressReport(srv, spreadsheetID)
		if errSheet != nil {
			log.Printf("Помилка отримання даних з Google Sheets (аркуш 'Звіт') для звіту ChatID %d: %v", chatID, errSheet)
			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Ваша Ціль:** `%.2f %s` за `%d днів` (встановлено %s).\n\n"+
					"⚠️ Не вдалося отримати дані з аркуша 'Звіт' для розрахунку прогресу:\n`%v`\n\n"+
					"🔥 %s",
				currentUserGoal.Amount, currentUserGoal.Currency, currentUserGoal.Days,
				currentUserGoal.SetDate.In(kyivLocation).Format("02.01.2006"),
				errSheet,
				motivation.GetRandomMotivation(),
			)
		} else {
			// Розрахунок прогресу
			amountAchievedSoFar := sheetData.Income
			remainingToAchieve := currentUserGoal.Amount - amountAchievedSoFar

			// Розрахунок календарних днів
			// currentUserGoal.SetDate зберігається в UTC, time.Since повертає тривалість
			// Для коректного розрахунку днів, що минули, беремо початок дня встановлення цілі та початок сьогоднішнього дня
			todayInKyiv := time.Now().In(kyivLocation)
			startDateOfGoalInKyiv := time.Date(currentUserGoal.SetDate.In(kyivLocation).Year(), currentUserGoal.SetDate.In(kyivLocation).Month(), currentUserGoal.SetDate.In(kyivLocation).Day(), 0, 0, 0, 0, kyivLocation)
			currentDateForCalc := time.Date(todayInKyiv.Year(), todayInKyiv.Month(), todayInKyiv.Day(), 0, 0, 0, 0, kyivLocation)
			
			daysPassedSinceGoalSet := int(currentDateForCalc.Sub(startDateOfGoalInKyiv).Hours() / 24)
			if daysPassedSinceGoalSet < 0 { daysPassedSinceGoalSet = 0 } // Якщо ціль на майбутнє

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
			
			// Розрахунок РОБОЧИХ днів, що залишилися
			goalEndDate := startDateOfGoalInKyiv.AddDate(0, 0, currentUserGoal.Days) // Кінець цілі - це початок дня SetDate + Days
			
			// Робочі дні рахуємо від сьогодні до кінця цілі включно
			// Якщо сьогоднішня дата вже після дати завершення цілі, то робочих днів 0
			var actualWorkingDaysLeft int
			var errWorkingDays error

			if currentDateForCalc.After(goalEndDate) { // Якщо сьогоднішня дата вже після дати завершення цілі
				actualWorkingDaysLeft = 0
				log.Printf("Підрахунок робочих днів: ціль вже мала завершитися (%s), робочих днів 0.", goalEndDate.Format("2006-01-02"))
			} else {
				// startDateForCount має бути сьогоднішньою датою (або завтрашньою, якщо сьогодні вже робочий і ми його не рахуємо для "залишилося")
				// Для простоти, рахуємо від сьогодні включно
				actualWorkingDaysLeft, errWorkingDays = sheets.CountWorkingDaysInRange(srv, spreadsheetID, currentDateForCalc, goalEndDate)
				if errWorkingDays != nil {
					log.Printf("Помилка підрахунку робочих днів для ChatID %d: %v. Будуть використані календарні дні.", chatID, errWorkingDays)
					actualWorkingDaysLeft = calendarDaysLeftForGoal // Відкат до календарних днів у разі помилки
				}
			}


			var requiredDailySmartStr string
			if remainingToAchieve <= 0 {
				requiredDailySmartStr = "0.00 (Ціль досягнуто!)"
			} else if actualWorkingDaysLeft <= 0 {
				requiredDailySmartStr = "∞ (Робочий час вийшов!)"
			} else {
				requiredDailySmart := remainingToAchieve / float64(actualWorkingDaysLeft)
				requiredDailySmartStr = fmt.Sprintf("%.2f", requiredDailySmart)
			}

			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Встановлена Ціль:**\n"+
					"   Сума: `%.2f %s`\n"+
					"   Загальний термін: `%d календарних днів`\n"+
					"   Встановлено: `%s`\n\n"+
					"📈 **Поточний Прогрес (з аркуша 'Звіт' станом на `%s`):\n"+
					"   Досягнуто (дохід): `%.2f %s`\n"+
					"   Залишилося досягти: `%.2f %s`\n"+
					"   Виконано: `%.2f %%`\n\n"+
					"⏳ **Час:**\n"+
					"   Календарних днів минуло: `%d` / `%d`\n"+
					"   Календарних днів залишилося: `%d`\n"+
					"   Заплановано робочих днів до кінця цілі: `%d`\n\n"+ // Новий рядок
					"💰 **Потрібно зараз (з урахуванням робочих днів):**\n"+
					"   Заробляти щодня: `%s %s`\n\n"+
					"🔥 %s",
				currentUserGoal.Amount, currentUserGoal.Currency,
				currentUserGoal.Days,
				currentUserGoal.SetDate.In(kyivLocation).Format("02.01.2006"),
				
				sheetData.Date, // Дата з аркуша "Звіт"
				
				amountAchievedSoFar, currentUserGoal.Currency,
				remainingToAchieve, currentUserGoal.Currency,
				progressPercentage,
				
				daysPassedSinceGoalSet, currentUserGoal.Days,
				calendarDaysLeftForGoal,
				actualWorkingDaysLeft, // Новий показник
				
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
