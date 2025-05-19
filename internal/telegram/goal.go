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
	// gsheets "google.golang.org/api/sheets/v4" // Не потрібен, якщо srv це *sheets.Service
)

// Тип FinancialGoal та функції SetUserGoal, GetUserGoal тепер в telegram.go

func HandleGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *sheets.Service, cfg *config.Config) { // Змінено тип srv
	chatID := message.Chat.ID
	inputText := message.Text
	log.Printf("Отримано текст для місячної цілі від чату %d: %s", chatID, inputText)

	re := regexp.MustCompile(`^(\d+(?:\.\d{1,2})?)\s*([а-яА-Яa-zA-Z]{3})?$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(inputText))

	var goal FinancialGoal 
	var parsedSuccessfully bool

	if len(matches) >= 3 { 
		amountStr := matches[1]
		currencyStr := strings.ToUpper(strings.TrimSpace(matches[2]))
		amount, errAmount := strconv.ParseFloat(amountStr, 64)

		if errAmount == nil {
			if currencyStr == "" { currencyStr = "UAH" }
			goal = FinancialGoal{
				Amount:       amount,
				Currency:     currencyStr,
				Days:         0, 
				OriginalText: inputText,
				SetDate:      time.Now().UTC(), 
			}
			parsedSuccessfully = true
		}
	}

	var responseText string
	if parsedSuccessfully {
		err := SetUserGoal(chatID, goal, srv, *cfg) // Передаємо *cfg, якщо SetUserGoal очікує config.Config
		if err != nil {
			responseText = fmt.Sprintf("⚠️ Помилка збереження цілі: %v", err)
			log.Printf("Помилка SetUserGoal для ChatID %d: %v", chatID, err)
		} else {
			responseText = fmt.Sprintf(
				"🎯 Ціль встановлено:\nСума: `%.2f %s`\n(Дата: `%s`)",
				goal.Amount, goal.Currency, goal.SetDate.In(KyivLocation).Format("02.01.2006"), 
			)
			log.Printf("Місячну ціль для %d збережено: %+v", chatID, goal)
		}
	} else {
		responseText = "⚠️ Не розпізнано формат цілі. Введіть: `СУМА ВАЛЮТА` (напр. `15000 грн`)."
		log.Printf("Помилка парсингу цілі для %d: '%s'", chatID, inputText)
	}

	msg := tgbotapi.NewMessage(chatID, responseText)
	msg.ParseMode = tgbotapi.ModeMarkdown 
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка надсилання відповіді HandleGoalInput для %d: %v", chatID, err)
	}
}

// Функція HandleMyGoalCommand з вашого файлу goal/goal.go (відповідь #74)
// Вона має бути тут, якщо викликається з handler.go як HandleMyGoalCommand
func HandleMyGoalCommand(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *sheets.Service, cfg *config.Config) {
    message := tgbotapi.NewMessage(msg.Chat.ID, "Введіть суму цілі, наприклад '1000 UAH'.")
    bot.Send(message)
    SetUserState(msg.Chat.ID, StateAwaitingGoalInput) // Встановлюємо стан
}
