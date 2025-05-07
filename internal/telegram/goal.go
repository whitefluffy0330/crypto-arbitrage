package telegram

import (
	"fmt"
	"log" 
	"regexp" 
	"strconv" 
	"strings" 
	"time"   

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Додаємо імпорти config та sheets
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // <<< ДОДАНО ІМПОРТ sheets
	gsheets "google.golang.org/api/sheets/v4" 
)

// HandleGoalInput обробляє введення користувачем тексту цілі.
func HandleGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *gsheets.Service, cfg config.Config) { 
	chatID := message.Chat.ID
	inputText := message.Text

	log.Printf("Отримано текст для цілі від чату %d: %s", chatID, inputText)

	re := regexp.MustCompile(`^(\d+(?:\.\d{1,2})?)\s*([а-яА-Яa-zA-Z]{3})?\s*,\s*(\d+)\s*(?i:(?:днів|дня|день))?$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(inputText))

	var goal FinancialGoal 
	var parsedSuccessfully bool

	if len(matches) >= 4 { 
		amountStr := matches[1]
		currencyStr := strings.ToUpper(strings.TrimSpace(matches[2]))
		daysStr := matches[3]
		amount, errAmount := strconv.ParseFloat(amountStr, 64)
		days, errDays := strconv.Atoi(daysStr)

		if errAmount == nil && errDays == nil && days > 0 {
			if currencyStr == "" { currencyStr = "UAH" }
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
		err := SetUserGoal(chatID, goal, srv, cfg) 
		if err != nil {
			responseText = fmt.Sprintf("⚠️ Помилка збереження цілі у Google Таблицю: %v", err)
			log.Printf("Помилка SetUserGoal для ChatID %d: %v", chatID, err)
		} else {
			// ВИПРАВЛЕНО: Використовуємо sheets.KyivLocation
			responseText = fmt.Sprintf(
				"🎯 Чудово! Вашу фінансову ціль встановлено та збережено:\n\n"+
					"Сума: `%.2f %s`\n"+
					"Термін: `%d днів`\n"+
					"Дата встановлення: `%s`",
				goal.Amount, goal.Currency, goal.Days, goal.SetDate.In(sheets.KyivLocation).Format("02.01.2006"), // <<< ЗМІНЕНО ТУТ
			)
			log.Printf("Ціль для чату %d успішно розпарсена, збережена: %+v", chatID, goal)
		}
	} else {
		responseText = "⚠️ Не вдалося розпізнати формат цілі...\n"+
		               "`СУМА [ВАЛЮТА], КІЛЬКІСТЬ_ДНІВ днів`\n\n"+
		               "Наприклад: `15000 грн, 30 днів`..."
		log.Printf("Помилка парсингу цілі для чату %d: '%s'", chatID, inputText)
	}

	msg := tgbotapi.NewMessage(chatID, responseText)
	msg.ParseMode = tgbotapi.ModeMarkdown 

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка надсилання відповіді HandleGoalInput для чату %d: %v", chatID, err)
	}
}

/*
// Закоментована функція HandleCallback 
func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) { ... }
*/
