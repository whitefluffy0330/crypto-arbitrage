package telegram

import (
	"fmt"
	"log"
	"strings" 

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

const (
	CallbackConfirmCloseGoal = "confirm_close_goal"
	CallbackCancelCloseGoal  = "cancel_close_goal"
)

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) { // Приймає cfg
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
				if strings.Contains(err.Error(), "не знайдено активної цілі") { 
					callbackText = "ℹ️ Активну ціль не знайдено."
					log.Printf("Спроба закрити неіснуючу ціль для ChatID: %d", chatID)
				} else {
					callbackText = "⚠️ Помилка закриття цілі."
					log.Printf("Помилка DeleteUserGoal для ChatID %d: %v", chatID, err)
				}
			} else {
				callbackText = "✅ Ціль успішно закрито!"
			}
			// Редагуємо повідомлення
			// Перевіряємо, чи є текст у вихідному повідомленні CallbackQuery
			originalText := ""
			if update.CallbackQuery.Message != nil {
				originalText = update.CallbackQuery.Message.Text
			}
			editText := tgbotapi.NewEditMessageText(chatID, messageID, originalText+"\n\n"+callbackText)
			bot.Send(editText)
			keyboard.ShowMainKeyboard(bot, chatID) 

		case CallbackCancelCloseGoal:
			log.Printf("Користувач %s (ChatID: %d) скасував закриття цілі.", userName, chatID)
			callbackText = "🚫 Закриття цілі скасовано."
			originalText := ""
			if update.CallbackQuery.Message != nil {
				originalText = update.CallbackQuery.Message.Text
			}
			editText := tgbotapi.NewEditMessageText(chatID, messageID, originalText+"\n\n"+callbackText)
			bot.Send(editText)
			keyboard.ShowMainKeyboard(bot, chatID)

		default:
			log.Printf("Передача CallbackQuery (Data: %s) в goal.HandleCallback для ChatID: %d", callbackData, chatID)
			// Передаємо cfg в goal.HandleCallback
			goal.HandleCallback(bot, update.CallbackQuery, srv, cfg) // <<< ПОТРІБНО ОНОВИТИ СИГНАТУРУ goal.HandleCallback
		}

		answerCallback := tgbotapi.NewCallback(update.CallbackQuery.ID, "") 
		if _, err := bot.Request(answerCallback); err != nil {
			log.Printf("Помилка відповіді на CallbackQuery: %v", err)
		}
		return
	}

	// Обробка звичайних повідомлень
	if update.Message == nil { return }

	chatID := update.Message.Chat.ID; msgText := update.Message.Text; userName := update.Message.From.UserName
	log.Printf("[%s] (%d): %s", userName, chatID, msgText)
	currentState := GetUserState(chatID)

	if currentState == StateAwaitingGoalInput {
		isCommandOrButton := false
		switch msgText { case "/start", "🔁 Старт", "/stop", "⛔️ Стоп", "/dayoff", "🏖 Вихідний", "/goal", "🎯 Моя ціль", "/closegoal", "❌ Закрити ціль", "/motivation", "/report", "📊 Прогрес": isCommandOrButton = true }

		if isCommandOrButton {
			log.Printf("Користувач %s в '%s', але надіслав команду '%s'. Скидаємо стан.", userName, currentState, msgText)
			SetUserState(chatID, StateDefault) 
		} else {
			log.Printf("Обробка від [%s] як введення цілі (стан: %s)", userName, currentState)
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
			log.Printf("Знайдено існуючу ціль для %d: %+v", chatID, currentGoal)
			// Формуємо текст повідомлення, ВИКОРИСТОВУЮЧИ currentGoal
			goalInfoText := fmt.Sprintf(
				"📌 Ваша поточна фінансова ціль:\n\n"+
					"Сума: `%.2f %s`\n"+
					"Термін: `%d днів`\n"+
					"Встановлено: `%s`\n\n"+
					"Щоб встановити нову ціль (вона перезапише поточну), надішліть її деталі у форматі `СУМА [ВАЛЮТА], КІЛЬКІСТЬ_ДНІВ днів` після цього повідомлення, або спочатку закрийте поточну ціль.",
				currentGoal.Amount, currentGoal.Currency, currentGoal.Days, currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006"), // Використовуємо sheets.KyivLocation
			)
			msg := tgbotapi.NewMessage(chatID, goalInfoText); msg.ParseMode = tgbotapi.ModeMarkdown
			if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання інфо про ціль: %v", err) } else { log.Printf("Повідомлення про поточну ціль надіслано для ChatID %d", chatID) }
			keyboard.ShowMainKeyboard(bot, chatID) 
		} else {
			log.Printf("Активна ціль для %d не знайдена.", chatID)
			goal.HandleMyGoalCommand(bot, chatID) 
			SetUserState(chatID, StateAwaitingGoalInput)
			log.Printf("Стан для %d встановлено в StateAwaitingGoalInput", chatID)
		}
	case "/closegoal", "❌ Закрити ціль": 
		log.Printf("Обробка команди /closegoal для ChatID: %d", chatID)
		currentGoal, exists := GetUserGoal(chatID, srv, cfg) // Передаємо cfg, currentGoal оголошено
		if exists {
			// Формуємо текст підтвердження, ВИКОРИСТОВУЮЧИ currentGoal
			confirmationText := fmt.Sprintf( 
				"❓ Ви впевнені, що хочете закрити поточну ціль?\n\n"+ 
					"Сума: `%.2f %s`\n"+
					"Термін: `%d днів`\n"+
					"Встановлено: `%s`",
				currentGoal.Amount, currentGoal.Currency, currentGoal.Days, currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006"), // Використовуємо sheets.KyivLocation
			)
			msg := tgbotapi.NewMessage(chatID, confirmationText); msg.ParseMode = tgbotapi.ModeMarkdown
			msg.ReplyMarkup = keyboard.CreateConfirmationKeyboard(CallbackConfirmCloseGoal, CallbackCancelCloseGoal)
			if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання запиту на підтвердження: %v", err) }
		} else {
			log.Printf("Немає активної цілі для закриття для %d", chatID)
			msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної цілі для закриття.")
			if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання повідомлення 'немає цілі': %v", err) }
			keyboard.ShowMainKeyboard(bot, chatID) 
		}
	case "/motivation":
		motivationText := motivation.GetRandomMotivation(); msg := tgbotapi.NewMessage(chatID, motivationText); if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання мотивації: %v", err) }
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/report", "📊 Прогрес":
		ReportProgress(bot, update.Message, srv, cfg) // Передаємо cfg
	default:
		log.Printf("Не розпізнана команда від [%s]: %s.", userName, msgText)
		keyboard.ShowMainKeyboard(bot, chatID)
	}
}
