package telegram

import (
	"fmt"
	"log"
	"regexp" // Для розбору тексту за допомогою регулярних виразів
	"strconv" // Для конвертації рядків у числа
	"strings" // Для роботи з рядками (наприклад, TrimSpace)
	"time"    // Для встановлення дати цілі

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ВАЖЛИВО: Оголошення 'var userGoals' було видалено звідси раніше.
// Воно тепер коректно визначене лише в файлі internal/telegram/telegram.go
// разом зі змінною userGoalsMutex, типом FinancialGoal та функціями SetUserGoal/GetUserGoal.

// HandleGoalInput обробляє введення користувачем тексту цілі,
// парсить його та зберігає структуровані дані.
func HandleGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID
	inputText := message.Text

	log.Printf("Отримано текст для цілі від чату %d: %s", chatID, inputText)

	// Регулярний вираз для парсингу формату "СУМА [ВАЛЮТА], КІЛЬКІСТЬ_ДНІВ [днів/дня/день]"
	// Приклади: "15000 грн, 30 днів", "500 USD, 60", "1000, 15 днів"
	// 1. Сума (число, може бути з копійками)
	// 2. Валюта (опціонально, 3 літери, наприклад, грн, usd, eur) - якщо немає, можна встановити за замовчуванням
	// 3. Кількість днів (число)
	// (?i) робить частину виразу нечутливою до регістру для "днів"
	re := regexp.MustCompile(`^(\d+(?:\.\d{1,2})?)\s*([а-яА-Яa-zA-Z]{3})?\s*,\s*(\d+)\s*(?i:(?:днів|дня|день))?$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(inputText))

	var goal FinancialGoal // Використовуємо структуру FinancialGoal з telegram.go
	var parsedSuccessfully bool

	if len(matches) >= 4 { // Очікуємо сам рядок + групи захоплення (сума, валюта(опц), дні)
		// Група matches[0] - це весь знайдений рядок
		// Група matches[1] - це сума
		// Група matches[2] - це валюта (може бути порожньою)
		// Група matches[3] - це дні

		amountStr := matches[1]
		currencyStr := strings.ToUpper(strings.TrimSpace(matches[2]))
		daysStr := matches[3]

		amount, errAmount := strconv.ParseFloat(amountStr, 64)
		days, errDays := strconv.Atoi(daysStr)

		if errAmount == nil && errDays == nil && days > 0 {
			if currencyStr == "" {
				currencyStr = "UAH" // Валюта за замовчуванням, якщо не вказано
			}

			goal = FinancialGoal{
				Amount:       amount,
				Currency:     currencyStr,
				Days:         days,
				OriginalText: inputText,
				SetDate:      time.Now().UTC(), // Зберігаємо час в UTC для універсальності
			}
			parsedSuccessfully = true
		}
	}

	var responseText string
	if parsedSuccessfully {
		SetUserGoal(chatID, goal) // Використовуємо функцію з telegram.go для збереження структурованої цілі
		responseText = fmt.Sprintf(
			"🎯 Чудово! Вашу фінансову ціль встановлено:\n\n"+
				"Сума: %.2f %s\n"+
				"Термін: %d днів\n"+
				"Дата встановлення: %s",
			goal.Amount, goal.Currency, goal.Days, goal.SetDate.Format("02.01.2006"), // Форматуємо дату
		)
		log.Printf("Ціль для чату %d успішно розпарсена та збережена: %+v", chatID, goal)
	} else {
		responseText = "⚠️ Не вдалося розпізнати формат цілі. Будь ласка, спробуйте ще раз у форматі:\n"+
		               "`СУМА [ВАЛЮТА], КІЛЬКІСТЬ_ДНІВ днів`\n\n"+
		               "Наприклад: `15000 грн, 30 днів` або `500 USD, 60 днів`.\n"+
		               "Валюта (3 літери) є опціональною (за замовчуванням UAH)."
		log.Printf("Помилка парсингу цілі для чату %d: вхідний текст '%s'", chatID, inputText)
	}

	msg := tgbotapi.NewMessage(chatID, responseText)
	// Встановлюємо ParseMode, якщо повідомлення містить Markdown (для повідомлення про помилку)
	if !parsedSuccessfully {
		msg.ParseMode = tgbotapi.ModeMarkdown
	}

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка надсилання відповіді HandleGoalInput для чату %d: %v", chatID, err)
	}
}

/*
// Закоментована функція HandleCallback (з попередньої версії цього файлу)
// ...
*/
