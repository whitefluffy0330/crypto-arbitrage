package telegram

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Додаємо імпорт gsheets для типу srv у сигнатурі HandleGoalInput
	gsheets "google.golang.org/api/sheets/v4"
)

// HandleGoalInput обробляє введення користувачем тексту цілі,
// парсить його та зберігає структуровані дані (в пам'яті та Google Sheets).
// Тепер приймає srv та spreadsheetID для передачі в SetUserGoal.
func HandleGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *gsheets.Service, spreadsheetID string) {
	chatID := message.Chat.ID
	inputText := message.Text

	log.Printf("Отримано текст для цілі від чату %d: %s", chatID, inputText)

	re := regexp.MustCompile(`^(\d+(?:\.\d{1,2})?)\s*([а-яА-Яa-zA-Z]{3})?\s*,\s*(\d+)\s*(?i:(?:днів|дня|день))?$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(inputText))

	var goal FinancialGoal // Використовуємо структуру FinancialGoal з telegram.go
	var parsedSuccessfully bool

	if len(matches) >= 4 {
		amountStr := matches[1]
		currencyStr := strings.ToUpper(strings.TrimSpace(matches[2]))
		daysStr := matches[3]

		amount, errAmount := strconv.ParseFloat(amountStr, 64)
		days, errDays := strconv.Atoi(daysStr)

		if errAmount == nil && errDays == nil && days > 0 {
			if currencyStr == "" {
				currencyStr = "UAH"
			}
			goal = FinancialGoal{
				Amount:       amount,
				Currency:     currencyStr,
				Days:         days,
				OriginalText: inputText,
				SetDate:      time.Now().UTC(),
			}
			parsedSuccessfully = true
		}
	}

	var responseText string
	if parsedSuccessfully {
		// Викликаємо оновлену SetUserGoal, передаючи srv та spreadsheetID, та обробляємо помилку
		err := SetUserGoal(chatID, goal, srv, spreadsheetID)
		if err != nil {
			// Якщо виникла помилка при збереженні в Google Sheet
			responseText = fmt.Sprintf("⚠️ Відбулася помилка під час збереження вашої цілі у Google Таблицю: %v\n\nВашу ціль поки що не збережено. Спробуйте пізніше або перевірте налаштування таблиці.", err)
			log.Printf("Помилка SetUserGoal для ChatID %d при збереженні в Google Sheet: %v", chatID, err)
		} else {
			// Якщо все успішно збережено (включно з Google Sheet)
			responseText = fmt.Sprintf(
				"🎯 Чудово! Вашу фінансову ціль встановлено та збережено:\n\n"+
					"Сума: `%.2f %s`\n"+
					"Термін: `%d днів`\n"+
					"Дата встановлення: `%s`",
				goal.Amount, goal.Currency, goal.Days, goal.SetDate.Format("02.01.2006"),
			)
			log.Printf("Ціль для чату %d успішно розпарсена, збережена в пам'яті та Google Sheets: %+v", chatID, goal)
		}
	} else {
		responseText = "⚠️ Не вдалося розпізнати формат цілі. Будь ласка, спробуйте ще раз у форматі:\n"+
		               "`СУМА [ВАЛЮТА], КІЛЬКІСТЬ_ДНІВ днів`\n\n"+
		               "Наприклад: `15000 грн, 30 днів` або `500 USD, 60 днів`.\n"+
		               "Валюта (3 літери) є опціональною (за замовчуванням UAH)."
		log.Printf("Помилка парсингу цілі для чату %d: вхідний текст '%s'", chatID, inputText)
	}

	msg := tgbotapi.NewMessage(chatID, responseText)
	// Встановлюємо ParseMode Markdown для всіх відповідей звідси,
	// оскільки і повідомлення про успіх, і про помилку можуть його використовувати.
	msg.ParseMode = tgbotapi.ModeMarkdown

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка надсилання відповіді HandleGoalInput для чату %d: %v", chatID, err)
	}
}

/*
// Закоментована функція HandleCallback (з попередньої версії цього файлу)
// ...
*/
