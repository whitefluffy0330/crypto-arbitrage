package telegram

import (
	"fmt" // Додано для форматування повідомлення про поточну ціль
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	gsheets "google.golang.org/api/sheets/v4"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" // Це підпакет goal
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
)

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, spreadsheetID string) {
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		log.Printf("Отримано CallbackQuery від [%s] (ChatID: %d), Data: %s", update.CallbackQuery.From.UserName, chatID, update.CallbackQuery.Data)
		goal.HandleCallback(bot, update.CallbackQuery) // Обробка з підпакета goal
		// Відповідь на CallbackQuery, щоб прибрати "годинник"
		callbackResp := tgbotapi.NewCallback(update.CallbackQuery.ID, "") // Можна додати текст відповіді
		if _, err := bot.AnswerCallbackQuery(callbackResp); err != nil {
			log.Printf("Помилка відповіді на CallbackQuery: %v", err)
		}
		return
	}

	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	msgText := update.Message.Text
	userName := update.Message.From.UserName

	log.Printf("[%s] (%d): %s", userName, chatID, msgText)

	currentState := GetUserState(chatID)

	if currentState == StateAwaitingGoalInput {
		log.Printf("Обробка повідомлення від [%s] як введення цілі (стан: %s)", userName, currentState)
		HandleGoalInput(bot, update.Message) // Викликаємо HandleGoalInput з goal.go (верхнього рівня)
		SetUserState(chatID, StateDefault)
		return
	}

	switch msgText {
	case "/start", "🔁 Старт":
		commands.StartWork(bot, update.Message)
	case "/stop", "⛔️ Стоп":
		commands.StopWork(bot, update.Message)
	case "/dayoff", "🏖 Вихідний":
		commands.DayOff(bot, update.Message, srv, spreadsheetID)
	case "/goal", "🎯 Моя ціль":
		currentGoal, exists := GetUserGoal(chatID) // Отримуємо поточну ціль
		if exists {
			// Якщо ціль існує, показуємо її
			goalInfoText := fmt.Sprintf(
				"📌 Ваша поточна фінансова ціль:\n\n"+
					"Сума: %.2f %s\n"+
					"Термін: %d днів\n"+
					"Встановлено: %s\n\n"+
					"Щоб встановити нову ціль (вона перезапише поточну), просто надішліть її деталі у форматі `СУМА [ВАЛЮТА], КІЛЬКІСТЬ_ДНІВ днів`.",
				currentGoal.Amount, currentGoal.Currency, currentGoal.Days, currentGoal.SetDate.Format("02.01.2006"),
			)
			msg := tgbotapi.NewMessage(chatID, goalInfoText)
			msg.ParseMode = tgbotapi.ModeMarkdown // Дозволяємо Markdown для форматування
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Помилка надсилання інформації про поточну ціль: %v", err)
			}
		}
		// Незалежно від того, чи існує стара ціль, пропонуємо встановити нову/оновити
		goal.HandleMyGoalCommand(bot, chatID) // Функція з підпакета goal, яка надсилає запит "Надішли свою ціль..."
		SetUserState(chatID, StateAwaitingGoalInput)

	case "/closegoal", "❌ Закрити ціль": // Додамо обробку кнопки пізніше
		currentGoal, exists := GetUserGoal(chatID)
		if exists {
			CloseUserGoal(bot, chatID) // CloseUserGoal викликає DeleteUserGoal і надсилає повідомлення
		} else {
			msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної цілі для закриття.")
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Помилка надсилання повідомлення 'немає цілі для закриття': %v", err)
			}
		}

	case "/motivation":
		motivationText := motivation.GetRandomMotivation()
		msg := tgbotapi.NewMessage(chatID, motivationText)
		if _, err := bot.Send(msg); err != nil {
			log.Printf("Помилка надсилання мотиваційного повідомлення для чату %d: %v", chatID, err)
		}
	case "/report", "📊 Прогрес":
		ReportProgress(bot, update.Message, srv, spreadsheetID)
	default:
		log.Printf("Не розпізнана команда або текст від [%s]: %s", userName, msgText)
		keyboard.ShowMainKeyboard(bot, chatID)
	}
}
