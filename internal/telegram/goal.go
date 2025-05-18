package telegram // Пакет той самий

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
// Використовує FinancialGoal та SetUserGoal з telegram.go
func HandleGoalInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *gsheets.Service, cfg config.Config) { 
	chatID := message.Chat.ID
	inputText := message.Text
	log.Printf("Отримано текст для місячної цілі від чату %d: %s", chatID, inputText)

	re := regexp.MustCompile(`^(\d+(?:\.\d{1,2})?)\s*([а-яА-Яa-zA-Z]{3})?$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(inputText))

	var goal FinancialGoal // Тип FinancialGoal тепер з telegram.go
	var parsedSuccessfully bool

	if len(matches) >= 3 { 
		amountStr := matches[1]
		currencyStr := strings.ToUpper(strings.TrimSpace(matches[2]))
		amount, errAmount := strconv.ParseFloat(amountStr, 64)

		if errAmount == nil {
			if currencyStr == "" {
				currencyStr = "UAH" 
			}
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
		err := SetUserGoal(chatID, goal, srv, cfg) // Функція SetUserGoal тепер з telegram.go
		if err != nil {
			responseText = fmt.Sprintf("⚠️ Помилка збереження цілі у Google Таблицю: %v", err)
			log.Printf("Помилка SetUserGoal для ChatID %d: %v", chatID, err)
		} else {
			responseText = fmt.Sprintf(
				"🎯 Чудово! Вашу ціль на поточний місяць встановлено:\n\n"+
					"Сума: `%.2f %s`\n"+
					"(Встановлено: `%s`)",
				goal.Amount, goal.Currency, goal.SetDate.In(KyivLocation).Format("02.01.2006"), // Використовуємо KyivLocation з telegram.go
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

// Якщо у вас був файл internal/telegram/goal/goal.go з функцією HandleMyGoalCommand,
// і вона потрібна, її можна залишити тут або перенести в handler.go.
// Наприклад, якщо HandleMyGoalCommand була такою:
/*
func HandleMyGoalCommand(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🎯 Введіть суму вашої цілі на поточний місяць (наприклад, `15000 ГРН` або `500 USD`). Валюта опціональна (за замовчуванням UAH).")
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка при відправці HandleMyGoalCommand (пакет telegram, файл goal.go): %v", err)
	}
}
*/
// Якщо HandleGoalCallback була у вашому файлі goal.go, вона може залишитися тут,
// або її логіку потрібно перенести в handler.go.
/*
func HandleGoalCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery, srv *gsheets.Service, cfg config.Config) { 
	// ... ваша логіка ...
}
*/
