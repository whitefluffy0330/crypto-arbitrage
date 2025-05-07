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

// ВАЖЛИВО: Оголошення 'var userGoals' ВИДАЛЕНО ЗВІДСИ.
// Воно тепер знаходиться в internal/telegram/telegram.go разом з userGoalsMutex,
// а також там визначено тип FinancialGoal та функції SetUserGoal/GetUserGoal.

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

	var goal FinancialGoal
	var parsedSuccessfully bool

	if len(matches) >= 4 { // Очікуємо сам рядок + 3 групи захоплення (сума, валюта, дні)
		amountStr := matches[1]
		currencyStr := strings.ToUpper(strings.TrimSpace(matches[2])) // Валюта, якщо є
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
				SetDate:      time.Now(),
			}
			parsedSuccessfully = true
		}
	}

	var responseText string
	if parsedSuccessfully {
		SetUserGoal(chatID, goal) // Використовуємо функцію з telegram.go для збереження
		responseText = fmt.Sprintf(
			"🎯 Чудово! Вашу фінансову ціль встановлено:\n\n"+
				"Сума: %.2f %s\n"+
				"Термін: %d днів\n"+
				"Дата встановлення: %s",
			goal.Amount, goal.Currency, goal.Days, goal.SetDate.Format("02.01.2006"),
		)
		log.Printf("Ціль для чату %d успішно розпарсена та збережена: %+v", chatID, goal)
	} else {
		responseText = "⚠️ Не вдалося розпізнати формат цілі. Будь ласка, спробуйте ще раз у форматі:\n"+
		               "`СУМА [ВАЛЮТА], КІЛЬКІСТЬ_ДНІВ днів`\n"+
		               "Наприклад: `15000 грн, 30 днів` або `500 USD, 60 днів`.\n"+
		               "Валюта є опціональною (за замовчуванням UAH) і має складатися з 3 літер."
		// Стан користувача StateAwaitingGoalInput вже скинуто на StateDefault у handler.go
		// після виклику цієї функції. Якщо ми хочемо, щоб користувач спробував ще раз
		// без повторного введення /goal, нам потрібно було б не скидати стан у handler.go
		// або встановлювати його тут знову. Поки що залишимо так для простоти.
		log.Printf("Помилка парсингу цілі для чату %d: вхідний текст '%s'", chatID, inputText)
	}

	msg := tgbotapi.NewMessage(chatID, responseText)
	if !parsedSuccessfully {
		msg.ParseMode = tgbotapi.ModeMarkdown // Для форматування повідомлення про помилку
	}

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка надсилання відповіді HandleGoalInput для чату %d: %v", chatID, err)
	}
}

/*
// Закоментована функція HandleCallback (з попередньої версії цього файлу)
// ... (якщо вона тут була, вона залишається закоментованою або видаленою) ...
*/
