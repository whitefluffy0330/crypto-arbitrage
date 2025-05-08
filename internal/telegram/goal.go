package telegram

import (
	"fmt"
	"log"
	"regexp" 
	"strconv" 
	"strings" 
	"time"   

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" 
	gsheets "google.golang.org/api/sheets/v4" 
)

// HandleGoalInput тепер парсить лише суму та валюту.
func HandleGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *gsheets.Service, cfg config.Config) { 
	chatID := message.Chat.ID
	inputText := message.Text
	log.Printf("Отримано текст для місячної цілі від чату %d: %s", chatID, inputText)

	// Регулярний вираз для парсингу "СУМА [ВАЛЮТА]"
	// Приклади: "15000 грн", "500 USD", "10000"
	re := regexp.MustCompile(`^(\d+(?:\.\d{1,2})?)\s*([а-яА-Яa-zA-Z]{3})?$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(inputText))

	var goal FinancialGoal 
	var parsedSuccessfully bool

	if len(matches) >= 3 { // Очікуємо сам рядок + група суми + опціональна група валюти
		amountStr := matches[1]
		currencyStr := strings.ToUpper(strings.TrimSpace(matches[2]))
		amount, errAmount := strconv.ParseFloat(amountStr, 64)

		if errAmount == nil {
			if currencyStr == "" {
				currencyStr = "UAH" // Валюта за замовчуванням
			}
			// Поле Days більше не релевантне для тривалості, встановлюємо 0
			goal = FinancialGoal{
				Amount:       amount,
				Currency:     currencyStr,
				Days:         0, // Або розрахувати кількість днів у поточному місяці? Поки що 0.
				OriginalText: inputText,
				SetDate:      time.Now().UTC(), // Залишаємо UTC для консистентності
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
			// Формуємо повідомлення для місячної цілі
			responseText = fmt.Sprintf(
				"🎯 Чудово! Вашу ціль на поточний місяць встановлено:\n\n"+
					"Сума: `%.2f %s`\n"+
					"(Встановлено: `%s`)",
				goal.Amount, goal.Currency, goal.SetDate.In(sheets.KyivLocation).Format("02.01.2006"),
			)
			log.Printf("Місячну ціль для чату %d успішно розпарсена та збережена: %+v", chatID, goal)
		}
	} else {
		responseText = "⚠️ Не вдалося розпізнати формат цілі. Будь ласка, введіть лише суму та, опціонально, валюту (3 літери):\n\n"+
		               "Наприклад: `15000 грн` або `500 USD`."
		log.Printf("Помилка парсингу місячної цілі для чату %d: '%s'", chatID, inputText)
	}

	msg := tgbotapi.NewMessage(chatID, responseText)
	msg.ParseMode = tgbotapi.ModeMarkdown 

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка надсилання відповіді HandleGoalInput для чату %d: %v", chatID, err)
	}
}

// Закоментована HandleCallback залишається без змін
/*
func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) { ... }
*/
