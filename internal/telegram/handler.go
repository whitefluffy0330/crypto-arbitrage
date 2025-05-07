package telegram

import (
	"fmt"
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
		
		// Обробка callback-запиту передається в підпакет goal
		goal.HandleCallback(bot, update.CallbackQuery) // Функція з internal/telegram/goal/goal.go

		// Відповідаємо на CallbackQuery, щоб прибрати індикатор завантаження на кнопці
		// Створюємо конфігурацію відповіді на callback
		callbackConfig := tgbotapi.NewCallback(update.CallbackQuery.ID, update.CallbackQuery.Data) // Можна додати текст, який побачить користувач
		// Надсилаємо запит через bot.Request()
		if _, err := bot.Request(callbackConfig); err != nil {
			log.Printf("Помилка відповіді на CallbackQuery: %v", err)
		}
		return // Завершуємо обробку цього оновлення
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
		switch msgText {
		case "/start", "🔁 Старт", "/stop", "⛔️ Стоп", "/dayoff", "🏖 Вихідний", "/goal", "🎯 Моя ціль", "/closegoal", "❌ Закрити ціль", "/motivation", "/report", "📊 Прогрес":
			log.Printf("Користувач %s (ChatID: %d) був у стані StateAwaitingGoalInput, але надіслав команду/натиснув кнопку '%s'. Скидаємо стан.", userName, chatID, msgText)
			SetUserState(chatID, StateDefault)
		default:
			log.Printf("Обробка повідомлення від [%s] як введення цілі (стан: %s)", userName, currentState)
			HandleGoalInput(bot, update.Message) // Викликаємо HandleGoalInput з goal.go (верхнього рівня)
			SetUserState(chatID, StateDefault)
			return 
		}
		currentState = GetUserState(chatID) 
	}

	switch msgText {
	case "/start", "🔁 Старт":
		commands.StartWork(bot, update.Message)
	case "/stop", "⛔️ Стоп":
		commands.StopWork(bot, update.Message)
	case "/dayoff", "🏖 Вихідний":
		commands.DayOff(bot, update.Message, srv, spreadsheetID)
	case "/goal", "🎯 Моя ціль":
		log.Printf("Обробка команди /goal для ChatID: %d", chatID)
		currentGoal, exists := GetUserGoal(chatID) // Отримуємо поточну ціль
		if exists {
			log.Printf("Для ChatID %d знайдено існуючу ціль: %+v", chatID, currentGoal) // Використання currentGoal
			goalInfoText := fmt.Sprintf(
				"📌 Ваша поточна фінансова ціль:\n\n"+
					"Сума: %.2f %s\n"+
					"Термін: %d днів\n"+
					"Встановлено: %s\n\n"+
					"Щоб встановити нову ціль (вона перезапише поточну), надішліть її деталі у форматі `СУМА [ВАЛЮТА], КІЛЬКІСТЬ_ДНІВ днів` після цього повідомлення.",
				currentGoal.Amount, currentGoal.Currency, currentGoal.Days, currentGoal.SetDate.Format("02.01.2006"), // Використання полів currentGoal
			)
			msg := tgbotapi.NewMessage(chatID, goalInfoText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Помилка надсилання інформації про поточну ціль для ChatID %d: %v", chatID, err)
			} else {
				log.Printf("Повідомлення про поточну ціль надіслано для ChatID %d", chatID)
			}
		} else {
			log.Printf("Для ChatID %d активна ціль не знайдена.", chatID)
		}
		goal.HandleMyGoalCommand(bot, chatID) 
		SetUserState(chatID, StateAwaitingGoalInput)
		log.Printf("Стан для ChatID %d встановлено в StateAwaitingGoalInput після /goal", chatID)

	case "/closegoal", "❌ Закрити ціль": 
		log.Printf("Обробка команди /closegoal для ChatID: %d", chatID)
		_, exists := GetUserGoal(chatID) // Перевіряємо, чи є ціль, перед тим як викликати CloseUserGoal
		if exists {
			log.Printf("Закриття існуючої цілі для ChatID %d", chatID)
			CloseUserGoal(bot, chatID) 
		} else {
			log.Printf("Немає активної цілі для закриття для ChatID %d", chatID)
			msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної цілі для закриття.")
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Помилка надсилання повідомлення 'немає цілі для закриття' для ChatID %d: %v", chatID, err)
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
		log.Printf("Не розпізнана команда або текст від [%s]: %s. Показано головну клавіатуру.", userName, msgText)
		keyboard.ShowMainKeyboard(bot, chatID)
	}
}
