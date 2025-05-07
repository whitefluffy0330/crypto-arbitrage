package telegram

import (
	"fmt"
	"log"
	"math"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Імпортуємо ваш пакет sheets для структури SheetRowData та функції GenerateProgressReport
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	// Імпортуємо пакет motivation
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

// ReportProgress надсилає звіт про прогрес користувачеві з розрахунками.
func ReportProgress(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, spreadsheetID string) {
	chatID := msg.Chat.ID
	var reportText string

	currentUserGoal, goalExists := GetUserGoal(chatID) // Отримуємо поточну ціль користувача

	if !goalExists {
		reportText = "🎯 Спочатку вам потрібно встановити фінансову ціль за допомогою команди /goal або кнопки \"🎯 Моя ціль\"."
		log.Printf("Запит звіту для ChatID %d, але ціль не встановлена.", chatID)
	} else {
		log.Printf("Генерація звіту для ChatID %d з ціллю: %+v", chatID, currentUserGoal)

		// Отримуємо дані з аркуша "Звіт"
		sheetData, err := sheets.GenerateProgressReport(srv, spreadsheetID)
		if err != nil {
			log.Printf("Помилка отримання даних з Google Sheets для звіту ChatID %d: %v", chatID, err)
			// Повідомляємо про помилку, але показуємо дані цілі
			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Ваша Ціль:** `%.2f %s` за `%d днів` (встановлено %s).\n\n"+
					"⚠️ Не вдалося отримати актуальні дані з Google Sheets для розрахунку прогресу:\n`%v`\n\n"+
					"🔥 %s", // Додаємо мотивацію навіть при помилці
				currentUserGoal.Amount, currentUserGoal.Currency, currentUserGoal.Days,
				currentUserGoal.SetDate.Format("02.01.2006"),
				err,
				motivation.GetRandomMotivation(),
			)
		} else {
			// Розрахунок прогресу на основі цілі та даних з таблиці
			
			// Припускаємо, що sheetData.Income (з комірки B2) - це сума, вже досягнута для цієї цілі
			amountAchievedSoFar := sheetData.Income 
			remainingToAchieve := currentUserGoal.Amount - amountAchievedSoFar

			// Розрахунок днів
			daysPassedSinceGoalSet := int(time.Since(currentUserGoal.SetDate).Hours() / 24)
			daysActuallyLeftForGoal := currentUserGoal.Days - daysPassedSinceGoalSet
			if daysActuallyLeftForGoal < 0 {
				daysActuallyLeftForGoal = 0 
			}

			// Розрахунок відсотка прогресу
			var progressPercentage float64
			if currentUserGoal.Amount > 0 { 
				progressPercentage = (amountAchievedSoFar / currentUserGoal.Amount) * 100
				if progressPercentage > 100 { progressPercentage = 100 } // Обмежимо 100%
				if progressPercentage < 0 { progressPercentage = 0 } // Не може бути менше 0
			}

			// Розрахунок необхідного щоденного заробітку з цього моменту
			var requiredDailyNow float64
			var requiredDailyNowStr string
			if remainingToAchieve <= 0 {
				requiredDailyNow = 0 
				requiredDailyNowStr = "0.00 (Ціль досягнуто!)"
			} else if daysActuallyLeftForGoal <= 0 {
				// Дні вийшли, ціль не досягнуто
				requiredDailyNow = math.Inf(1) 
				requiredDailyNowStr = "∞ (Час вийшов!)"
			} else {
				requiredDailyNow = remainingToAchieve / float64(daysActuallyLeftForGoal)
				requiredDailyNowStr = fmt.Sprintf("%.2f", requiredDailyNow)
			}

			// Формуємо звіт з розрахунками
			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Встановлена Ціль:**\n"+
					"   `%.2f %s` за `%d днів` (з %s)\n\n"+
					"📈 **Прогрес:**\n"+
					"   Досягнуто (з таблиці `Звіт!B2`): `%.2f %s`\n"+
					"   Залишилося досягти: `%.2f %s`\n"+
					"   Виконано: `%.2f %%`\n\n"+
					"⏳ **Час:**\n"+
					"   Днів минуло: `%d` / `%d`\n"+
					"   Залишилося днів: `%d`\n\n"+
					"💰 **Потрібно зараз:**\n"+
					"   Заробляти щодня: `%s %s`\n\n"+
					"🔥 %s",
				currentUserGoal.Amount, currentUserGoal.Currency, currentUserGoal.Days, currentUserGoal.SetDate.Format("02.01.2006"),
				
				amountAchievedSoFar, currentUserGoal.Currency, // Показуємо досягнуту суму
				remainingToAchieve, currentUserGoal.Currency, // Показуємо залишок
				progressPercentage, // Показуємо відсоток
				
				daysPassedSinceGoalSet, currentUserGoal.Days, // Минуло / Всього днів
				daysActuallyLeftForGoal, // Залишилося днів
				
				requiredDailyNowStr, currentUserGoal.Currency, // Скільки потрібно заробляти
				
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
