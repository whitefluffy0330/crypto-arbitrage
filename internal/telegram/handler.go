package telegram

import (
	"fmt"
	"log"
	"strings" // Потрібен для Callback обробки

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Додаємо імпорт config
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" // Підпакет goal
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

// Константи для callback даних
const (
	CallbackConfirmCloseGoal = "confirm_close_goal"
	CallbackCancelCloseGoal  = "cancel_close_goal"
)

// HandleUpdate тепер приймає cfg config.Config
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	// Обробка CallbackQuery
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		messageID := update.CallbackQuery.Message.MessageID
		userName := update.CallbackQuery.From.UserName
		callbackData := update.CallbackQuery.Data

		log.Printf("Отримано CallbackQuery від [%s] (ChatID: %d), Data: %s, MessageID: %d", userName, chatID, callbackData, messageID)

		var callbackText string 

		switch callbackData {
		case CallbackConfirmCloseGoal:
			log.Printf("Користувач %s (ChatID: %d) підтвердив закриття цілі.", userName, chatID)
			err := DeleteUserGoal(chatID, srv, cfg) // Передаємо cfg
			if err != nil {
				if strings.Contains(err.Error(), "не знайдено активної цілі") { // Перевірка помилки
					callbackText = "ℹ️ Активну ціль не знайдено. Можливо, її вже було закрито."
					log.Printf("Спроба закрити неіснуючу/вже закриту ціль для ChatID: %d", chatID)
				} else {
					callbackText = "⚠️ Помилка закриття цілі. Спробуйте пізніше."
					log.Printf("Помилка DeleteUserGoal для ChatID %d: %v", chatID, err)
				}
			} else {
				callbackText = "✅ Ціль успішно закрито!"
			}
			editText := tgbotapi.NewEditMessageText(chatID, messageID, update.CallbackQuery.Message.Text+"\n\n"+callbackText)
			bot.Send(editText)
			keyboard.ShowMainKeyboard(bot, chatID) 

		case CallbackCancelCloseGoal:
			log.Printf("Користувач %s (ChatID: %d) скасував закриття цілі.", userName, chatID)
			callbackText = "🚫 Закриття цілі скасовано."
			editText := tgbotapi.NewEditMessageText(chatID, messageID, update.CallbackQuery.Message.Text+"\n\n"+callbackText)
			bot.Send(editText)
			keyboard.ShowMainKeyboard(bot, chatID)

		default:
			log.Printf("Передача CallbackQuery (Data: %s) в goal.HandleCallback для ChatID: %d", callbackData, chatID)
			// Передаємо cfg в goal.HandleCallback (потрібно оновити і його сигнатуру)
			goal.HandleCallback(bot, update.CallbackQuery, srv, cfg) // <<< ОНОВИТИ СИГНАТУРУ goal.HandleCallback!
		}

		answerCallback := tgbotapi.NewCallback(update.CallbackQuery.ID, "") 
		if _, err := bot.Request(answerCallback); err != nil {
			log.Printf("Помилка відповіді на CallbackQuery: %v", err)
		}
		return
	}

	// Обробка звичайних повідомлень
	if update.Message == nil { return }

	chatID := update.Message.Chat.ID
	msgText := update.Message.Text
	userName := update.Message.From.UserName
	log.Printf("[%s] (%d): %s", userName, chatID, msgText)

	currentState := GetUserState(chatID)

	if currentState == StateAwaitingGoalInput {
		isCommandOrButton := false
		switch msgText { case "/start", "🔁 Старт", "/stop", "⛔️ Стоп", "/dayoff", "🏖 Вихідний", "/goal", "🎯 Моя ціль", "/closegoal", "❌ Закрити ціль", "/motivation", "/report", "📊 Прогрес": isCommandOrButton = true }

		if isCommandOrButton {
			log.Printf("Користувач %s (ChatID: %d) в '%s', але надіслав команду '%s'. Скидаємо стан.", userName, chatID, currentState, msgText)
			SetUserState(chatID, StateDefault) 
		} else {
			log.Printf("Обробка повідомлення від [%s] як введення цілі (стан: %s)", userName, currentState)
			HandleGoalInput(bot, update.Message, srv, cfg) // Передаємо cfg
			SetUserState(chatID, StateDefault)
			keyboard.ShowMainKeyboard(bot, chatID)
			return 
		}
		currentState = GetUserState(chatID) 
	}

	switch msgText {
	case "/start", "🔁 Старт":
		commands.StartWork(bot, update.Message, srv, cfg) // Передаємо cfg
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/stop", "⛔️ Стоп":
		commands.StopWork(bot, update.Message, srv, cfg) // Передаємо cfg
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/dayoff", "🏖 Вихідний":
		commands.DayOff(bot, update.Message, srv, cfg) // Передаємо cfg
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/goal", "🎯 Моя ціль":
		log.Printf("Обробка команди /goal для ChatID: %d", chatID)
		currentGoal, exists := GetUserGoal(chatID, srv, cfg) // Передаємо cfg
		if exists {
			log.Printf("Для ChatID %d знайдено існуючу ціль: %+v", chatID, currentGoal)
			goalInfoText := fmt.Sprintf( /* ... текст ... */ )
			msg := tgbotapi.NewMessage(chatID, goalInfoText); msg.ParseMode = tgbotapi.ModeMarkdown
			if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання інфо про ціль: %v", err) } else { log.Printf("Повідомлення про поточну ціль надіслано для ChatID %d", chatID) }
			keyboard.ShowMainKeyboard(bot, chatID) // Показуємо меню після відображення
		} else {
			log.Printf("Для ChatID %d активна ціль не знайдена.", chatID)
			goal.HandleMyGoalCommand(bot, chatID) 
			SetUserState(chatID, StateAwaitingGoalInput)
			log.Printf("Стан для ChatID %d встановлено в StateAwaitingGoalInput після /goal", chatID)
		}
	case "/closegoal", "❌ Закрити ціль": 
		log.Printf("Обробка команди /closegoal для ChatID: %d", chatID)
		currentGoal, exists := GetUserGoal(chatID, srv, cfg) // Передаємо cfg
		if exists {
			confirmationText := fmt.Sprintf( /* ... текст підтвердження ... */ )
			msg := tgbotapi.NewMessage(chatID, confirmationText); msg.ParseMode = tgbotapi.ModeMarkdown
			msg.ReplyMarkup = keyboard.CreateConfirmationKeyboard(CallbackConfirmCloseGoal, CallbackCancelCloseGoal)
			if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання запиту на підтвердження: %v", err) }
		} else {
			log.Printf("Немає активної цілі для закриття для ChatID %d", chatID)
			msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної цілі для закриття.")
			if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання повідомлення 'немає цілі': %v", err) }
			keyboard.ShowMainKeyboard(bot, chatID) 
		}
	case "/motivation":
		motivationText := motivation.GetRandomMotivation()
		msg := tgbotapi.NewMessage(chatID, motivationText); if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання мотивації: %v", err) }
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/report", "📊 Прогрес":
		ReportProgress(bot, update.Message, srv, cfg) // Передаємо cfg
	default:
		log.Printf("Не розпізнана команда від [%s]: %s.", userName, msgText)
		keyboard.ShowMainKeyboard(bot, chatID)
	}
}
