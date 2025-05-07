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

	// Тепер передаємо srv та spreadsheetID в GetUserGoal
	currentUserGoal, goalExists := GetUserGoal(chatID, srv, spreadsheetID) // <<< ЗМІНЕНО ТУТ

	if !goalExists {
		reportText = "🎯 Спочатку вам потрібно встановити фінансову ціль за допомогою команди /goal або кнопки \"🎯 Моя ціль\"."
		log.Printf("Запит звіту для ChatID %d, але ціль не встановлена.", chatID)
	} else {
		log.Printf("Генерація звіту для ChatID %d з ціллю: %+v", chatID, currentUserGoal)

		sheetData, err := sheets.GenerateProgressReport(srv, spreadsheetID)
		if err != nil {
			log.Printf("Помилка отримання даних з Google Sheets для звіту ChatID %d: %v", chatID, err)
			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Ваша Ціль:** `%.2f %s` за `%d днів` (встановлено %s).\n\n"+
					"⚠️ Не вдалося отримати актуальні дані з Google Sheets для розрахунку прогресу:\n`%v`\n\n"+
					"🔥 %s", 
				currentUserGoal.Amount, currentUserGoal.Currency, currentUserGoal.Days,
				currentUserGoal.SetDate.In(kyivLocation).Format("02.01.2006"), // Додано In(kyivLocation)
				err,
				motivation.GetRandomMotivation(),
			)
		} else {
			amountAchievedSoFar := sheetData.Income 
			remainingToAchieve := currentUserGoal.Amount - amountAchievedSoFar

			// Перевіряємо, чи SetDate не є нульовим часом перед розрахунком
			var daysPassedSinceGoalSet int
			if !currentUserGoal.SetDate.IsZero() {
				daysPassedSinceGoalSet = int(time.Since(currentUserGoal.SetDate.UTC()).Hours() / 24) // Розраховуємо різницю від UTC дати встановлення
			} else {
				daysPassedSinceGoalSet = 0 // Або якесь значення за замовчуванням
			}

			daysActuallyLeftForGoal := currentUserGoal.Days - daysPassedSinceGoalSet
			if daysActuallyLeftForGoal < 0 {
				daysActuallyLeftForGoal = 0 
			}

			var progressPercentage float64
			if currentUserGoal.Amount > 0 { 
				progressPercentage = (amountAchievedSoFar / currentUserGoal.Amount) * 100
				if progressPercentage > 100 { progressPercentage = 100 } 
				if progressPercentage < 0 { progressPercentage = 0 } 
			}

			var requiredDailyNow float64
			var requiredDailyNowStr string
			if remainingToAchieve <= 0 {
				requiredDailyNow = 0 
				requiredDailyNowStr = "0.00 (Ціль досягнуто!)"
			} else if daysActuallyLeftForGoal <= 0 {
				requiredDailyNow = math.Inf(1) 
				requiredDailyNowStr = "∞ (Час вийшов!)"
			} else {
				requiredDailyNow = remainingToAchieve / float64(daysActuallyLeftForGoal)
				requiredDailyNowStr = fmt.Sprintf("%.2f", requiredDailyNow)
			}

			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Встановлена Ціль:**\n"+
					"   Сума: `%.2f %s`\n"+
					"   Загальний термін: `%d днів`\n"+
					"   Встановлено: `%s`\n\n"+ // Відображаємо в локальному часі
					"📈 **Поточний Прогрес (на основі даних з аркуша '%s' станом на '%s'):**\n"+
					"   Досягнуто (з таблиці `Звіт!B2`): `%.2f %s`\n"+
					"   Залишилося досягти: `%.2f %s`\n"+
					"   Виконано: `%.2f %%`\n\n"+
					"⏳ **Час:**\n"+
					"   Днів минуло: `%d` / `%d`\n"+
					"   Залишилося днів: `%d`\n\n"+
					"💰 **Потрібно зараз:**\n"+
					"   Заробляти щодня: `%s %s`\n"+
					"   (Дані з таблиці 'Потрібно щодня': `%.2f`)\n\n"+ // Порівняння з даними з таблиці 'Звіт!E2'
					"🔥 %s",
				currentUserGoal.Amount, currentUserGoal.Currency,
				currentUserGoal.Days,
				currentUserGoal.SetDate.In(kyivLocation).Format("02.01.2006"), // Додано In(kyivLocation)
				
				"Звіт", 
				sheetData.Date, 
				
				amountAchievedSoFar, currentUserGoal.Currency, 
				remainingToAchieve, currentUserGoal.Currency,
				progressPercentage, 
				
				daysPassedSinceGoalSet, currentUserGoal.Days, 
				daysActuallyLeftForGoal, 
				
				requiredDailyNowStr, currentUserGoal.Currency,
				sheetData.SheetReqDaily, 
				
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
