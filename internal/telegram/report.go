package telegram

import (
	"fmt"
	"log"
	"math" // Для округлення та інших математичних операцій
	"time" // Для розрахунку днів, що минули/залишились

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Імпортуємо ваш пакет sheets для структури SheetRowData та функції GenerateProgressReport
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	// Імпортуємо пакет motivation
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4" // Використовуємо gsheets для типу srv *gsheets.Service
)

// ReportProgress надсилає звіт про прогрес користувачеві.
func ReportProgress(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *gsheets.Service, spreadsheetID string) {
	chatID := msg.Chat.ID
	var reportText string

	currentUserGoal, goalExists := GetUserGoal(chatID) // Отримуємо поточну ціль користувача

	if !goalExists {
		reportText = "🎯 Спочатку вам потрібно встановити фінансову ціль за допомогою команди /goal або кнопки \"🎯 Моя ціль\"."
		log.Printf("Запит звіту для ChatID %d, але ціль не встановлена.", chatID)
	} else {
		log.Printf("Генерація звіту для ChatID %d з ціллю: %+v", chatID, currentUserGoal)

		// Отримуємо структуровані дані з Google Sheets
		sheetData, err := sheets.GenerateProgressReport(srv, spreadsheetID)
		if err != nil {
			log.Printf("Помилка отримання даних з Google Sheets для звіту ChatID %d: %v", chatID, err)
			reportText = fmt.Sprintf("📊 **Звіт про Прогрес** 📊\n\n"+
				"🎯 **Ваша Ціль:** %.2f %s за %d днів (встановлено %s).\n\n"+
				"⚠️ Не вдалося отримати дані з Google Sheets для розрахунку прогресу: %v",
				currentUserGoal.Amount, currentUserGoal.Currency, currentUserGoal.Days,
				currentUserGoal.SetDate.Format("02.01.2006"),
				err,
			)
		} else {
			// Розрахунок прогресу
			amountAchievedSoFar := sheetData.Income // Припускаємо, що Income з таблиці - це те, що вже досягнуто
			remainingToAchieve := currentUserGoal.Amount - amountAchievedSoFar

			daysPassedSinceGoalSet := int(time.Since(currentUserGoal.SetDate).Hours() / 24)
			daysActuallyLeftForGoal := currentUserGoal.Days - daysPassedSinceGoalSet
			if daysActuallyLeftForGoal < 0 {
				daysActuallyLeftForGoal = 0 // Не може бути менше 0
			}

			var progressPercentage float64
			if currentUserGoal.Amount > 0 { // Уникаємо ділення на нуль
				progressPercentage = (amountAchievedSoFar / currentUserGoal.Amount) * 100
			}

			var requiredDailyNow float64
			if daysActuallyLeftForGoal > 0 && remainingToAchieve > 0 {
				requiredDailyNow = remainingToAchieve / float64(daysActuallyLeftForGoal)
			} else if remainingToAchieve <= 0 {
				requiredDailyNow = 0 // Ціль досягнуто
			} else {
				requiredDailyNow = math.Inf(1) // Дні вийшли, ціль не досягнуто
			}

			// Формуємо звіт
			reportText = fmt.Sprintf(
				"📊 **Ваш Звіт про Прогрес** 📊\n\n"+
					"🎯 **Встановлена Ціль:**\n"+
					"   Сума: `%.2f %s`\n"+
					"   Загальний термін: `%d днів`\n"+
					"   Встановлено: `%s`\n\n"+
					"📈 **Поточний Прогрес (на основі даних з аркуша '%s' станом на '%s'):**\n"+
					"   Досягнуто (дохід з таблиці): `%.2f %s`\n"+
					"   Залишилося досягти: `%.2f %s`\n"+
					"   Прогрес: `%.2f%%`\n\n"+
					"⏳ **Час:**\n"+
					"   Днів минуло з моменту встановлення цілі: `%d`\n"+
					"   Залишилося днів для досягнення цілі: `%d`\n\n"+
					"💰 **Щоденні показники (розрахункові):**\n"+
					"   Необхідно заробляти щодня для досягнення цілі: `%.2f %s`\n"+
					"   (Дані з таблиці 'Потрібно щодня': `%.2f`)\n\n"+
					"🔥 %s",
				currentUserGoal.Amount, currentUserGoal.Currency,
				currentUserGoal.Days,
				currentUserGoal.SetDate.Format("02.01.2006"),
				"Звіт", // Назва аркуша, звідки дані (можна зробити динамічним, якщо readRange не константа)
				sheetData.Date, // Дата з таблиці
				amountAchievedSoFar, currentUserGoal.Currency, // Використовуємо валюту цілі
				remainingToAchieve, currentUserGoal.Currency,
				progressPercentage,
				daysPassedSinceGoalSet,
				daysActuallyLeftForGoal,
				requiredDailyNow, currentUserGoal.Currency,
				sheetData.SheetReqDaily, // Порівняння з тим, що в таблиці
				motivation.GetRandomMotivation(), // Додаємо мотиваційну фразу
			)
		}
	}

	response := tgbotapi.NewMessage(chatID, reportText)
	response.ParseMode = tgbotapi.ModeMarkdown // Використовуємо Markdown для форматування
	if _, err := bot.Send(response); err != nil {
		log.Printf("Помилка надсилання звіту про прогрес для чату %d: %v", chatID, err)
	}
}
