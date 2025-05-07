package telegram

import (
	"fmt"
	"log"
	"strings" // Додано для перевірки callback даних

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	gsheets "google.golang.org/api/sheets/v4"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" // Це підпакет goal
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
)

const (
	CallbackConfirmCloseGoal = "confirm_close_goal"
	CallbackCancelCloseGoal  = "cancel_close_goal"
)

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, spreadsheetID string) {
	// Обробка CallbackQuery
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		messageID := update.CallbackQuery.Message.MessageID
		userName := update.CallbackQuery.From.UserName
		callbackData := update.CallbackQuery.Data

		log.Printf("Отримано CallbackQuery від [%s] (ChatID: %d), Data: %s, MessageID: %d", userName, chatID, callbackData, messageID)

		var callbackText string // Текст для сповіщення користувача після натискання кнопки

		switch callbackData {
		case CallbackConfirmCloseGoal:
			log.Printf("Користувач %s (ChatID: %d) підтвердив закриття цілі.", userName, chatID)
			err := DeleteUserGoal(chatID, srv, spreadsheetID) // Викликаємо функцію з telegram.go
			if err != nil {
				if strings.Contains(err.Error(), "не знайдено активної цілі для оновлення") {
					callbackText = "ℹ️ Активну ціль не знайдено. Можливо, її вже було закрито."
					log.Printf("Спроба закрити неіснуючу/вже закриту ціль для ChatID: %d", chatID)
				} else {
					callbackText = "⚠️ Помилка закриття цілі. Спробуйте пізніше."
					log.Printf("Помилка DeleteUserGoal для ChatID %d: %v", chatID, err)
				}
			} else {
				callbackText = "✅ Ціль успішно закрито!"
			}
			// Редагуємо повідомлення, щоб прибрати кнопки
			editText := tgbotapi.NewEditMessageText(chatID, messageID, update.CallbackQuery.Message.Text+"\n\n"+callbackText)
			bot.Send(editText)
			keyboard.ShowMainKeyboard(bot, chatID) // Показуємо головне меню

		case CallbackCancelCloseGoal:
			log.Printf("Користувач %s (ChatID: %d) скасував закриття цілі.", userName, chatID)
			callbackText = "🚫 Закриття цілі скасовано."
			// Редагуємо повідомлення, щоб прибрати кнопки
			editText := tgbotapi.NewEditMessageText(chatID, messageID, update.CallbackQuery.Message.Text+"\n\n"+callbackText)
			bot.Send(editText)
			keyboard.ShowMainKeyboard(bot, chatID) // Показуємо головне меню

		default:
			// Якщо це інший callback, передаємо його в підпакет goal
			// Передаємо srv та spreadsheetID, оскільки HandleCallback в підпакеті goal тепер їх очікує
			log.Printf("Передача CallbackQuery (Data: %s) в goal.HandleCallback для ChatID: %d", callbackData, chatID)
			goal.HandleCallback(bot, update.CallbackQuery, srv, spreadsheetID)
		}

		// Відповідаємо на сам CallbackQuery, щоб прибрати "годинник" на кнопці
		// Якщо відповідь вже була надіслана через EditMessageText, текст тут може бути порожнім
		answerCallback := tgbotapi.NewCallback(update.CallbackQuery.ID, "") 
		if _, err := bot.Request(answerCallback); err != nil {
			log.Printf("Помилка відповіді на CallbackQuery: %v", err)
		}
		return // Завершуємо обробку цього оновлення
	}

	// Обробка звичайних повідомлень
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	msgText := update.Message.Text
	userName := update.Message.From.UserName

	log.Printf("[%s] (%d): %s", userName, chatID, msgText)

	currentState := GetUserState(chatID)

	if currentState == StateAwaitingGoalInput {
		isCommandOrButton := false
		switch msgText {
		case "/start", "🔁 Старт", "/stop", "⛔️ Стоп", "/dayoff", "🏖 Вихідний", "/goal", "🎯 Моя ціль", "/closegoal", "❌ Закрити ціль", "/motivation", "/report", "📊 Прогрес":
			isCommandOrButton = true
		}

		if isCommandOrButton {
			log.Printf("Користувач %s (ChatID: %d) був у стані StateAwaitingGoalInput, але надіслав команду/натиснув кнопку '%s'. Скидаємо стан.", userName, chatID, msgText)
			SetUserState(chatID, StateDefault)
		} else {
			log.Printf("Обробка повідомлення від [%s] як введення цілі (стан: %s)", userName, currentState)
			HandleGoalInput(bot, update.Message, srv, spreadsheetID)
			SetUserState(chatID, StateDefault)
			keyboard.ShowMainKeyboard(bot, chatID)
			return
		}
		currentState = GetUserState(chatID)
	}

	switch msgText {
	case "/start", "🔁 Старт":
		commands.StartWork(bot, update.Message, srv, spreadsheetID)
		keyboard.ShowMainKeyboard(bot, chatID)
	case "/stop", "⛔️ Стоп":
		commands.StopWork(bot, update.Message, srv, spreadsheetID)
		keyboard.ShowMainKeyboard(bot, chatID)
	case "/dayoff", "🏖 Вихідний":
		commands.DayOff(bot, update.Message, srv, spreadsheetID)
		keyboard.ShowMainKeyboard(bot, chatID)
	case "/goal", "🎯 Моя ціль":
		log.Printf("Обробка команди /goal для ChatID: %d", chatID)
		currentGoal, exists := GetUserGoal(chatID, srv, spreadsheetID)
		if exists {
			log.Printf("Для ChatID %d знайдено існуючу ціль: %+v", chatID, currentGoal)
			goalInfoText := fmt.Sprintf(
				"📌 Ваша поточна фінансова ціль:\n\n"+
					"Сума: `%.2f %s`\n"+
					"Термін: `%d днів`\n"+
					"Встановлено: `%s`\n\n"+
					"Щоб встановити нову ціль (вона перезапише поточну), надішліть її деталі у форматі `СУМА [ВАЛЮТА], КІЛЬКІСТЬ_ДНІВ днів` після цього повідомлення, або спочатку закрийте поточну ціль.",
				currentGoal.Amount, currentGoal.Currency, currentGoal.Days, currentGoal.SetDate.In(kyivLocation).Format("02.01.2006"),
			)
			msg := tgbotapi.NewMessage(chatID, goalInfoText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Помилка надсилання інформації про поточну ціль: %v", err)
			} else {
				log.Printf("Повідомлення про поточну ціль надіслано для ChatID %d", chatID)
			}
			// Після показу поточної цілі, пропонуємо встановити нову АБО показуємо головне меню
			goal.HandleMyGoalCommand(bot, chatID) // Запит на введення нової цілі
			SetUserState(chatID, StateAwaitingGoalInput)
			log.Printf("Стан для ChatID %d встановлено в StateAwaitingGoalInput після /goal (ціль існувала)", chatID)
		} else {
			log.Printf("Для ChatID %d активна ціль не знайдена. Пропонуємо встановити.", chatID)
			goal.HandleMyGoalCommand(bot, chatID)
			SetUserState(chatID, StateAwaitingGoalInput)
			log.Printf("Стан для ChatID %d встановлено в StateAwaitingGoalInput після /goal (ціль не існувала)", chatID)
		}

	case "/closegoal", "❌ Закрити ціль":
		log.Printf("Обробка команди /closegoal для ChatID: %d", chatID)
		currentGoal, exists := GetUserGoal(chatID, srv, spreadsheetID)
		if exists {
			confirmationText := fmt.Sprintf(
				"❓ Ви впевнені, що хочете закрити поточну ціль?\n\n"+
					"Сума: `%.2f %s`\n"+
					"Термін: `%d днів`\n"+
					"Встановлено: `%s`",
				currentGoal.Amount, currentGoal.Currency, currentGoal.Days, currentGoal.SetDate.In(kyivLocation).Format("02.01.2006"),
			)
			msg := tgbotapi.NewMessage(chatID, confirmationText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			// Створюємо inline-клавіатуру для підтвердження
			msg.ReplyMarkup = keyboard.CreateConfirmationKeyboard(CallbackConfirmCloseGoal, CallbackCancelCloseGoal)
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Помилка надсилання запиту на підтвердження закриття цілі: %v", err)
			}
		} else {
			log.Printf("Немає активної цілі для закриття для ChatID %d", chatID)
			msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної цілі для закриття.")
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Помилка надсилання повідомлення 'немає цілі для закриття': %v", err)
			}
			keyboard.ShowMainKeyboard(bot, chatID) // Показуємо головне меню, якщо немає чого закривати
		}

	case "/motivation":
		motivationText := motivation.GetRandomMotivation()
		msg := tgbotapi.NewMessage(chatID, motivationText)
		if _, err := bot.Send(msg); err != nil {
			log.Printf("Помилка надсилання мотиваційного повідомлення: %v", err)
		}
		keyboard.ShowMainKeyboard(bot, chatID)
	case "/report", "📊 Прогрес":
		ReportProgress(bot, update.Message, srv, spreadsheetID)
		// ReportProgress сам надсилає повідомлення, клавіатуру тут можна не показувати,
		// або показати після звіту, якщо це доречно.
		// keyboard.ShowMainKeyboard(bot, chatID) 
	default:
		log.Printf("Не розпізнана команда або текст від [%s]: %s. Показано головну клавіатуру.", userName, msgText)
		keyboard.ShowMainKeyboard(bot, chatID)
	}
}
