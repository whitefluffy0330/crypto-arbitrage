package telegram

import (
	"fmt"
	"log"
	"strings" 

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" // Підпакет goal
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

const (
	CallbackConfirmCloseGoal = "confirm_close_goal"
	CallbackCancelCloseGoal  = "cancel_close_goal"
)

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) { 
	// Обробка CallbackQuery
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID; messageID := update.CallbackQuery.Message.MessageID; userName := update.CallbackQuery.From.UserName; callbackData := update.CallbackQuery.Data
		log.Printf("Callback від [%s](%d): Data=%s, MsgID=%d", userName, chatID, callbackData, messageID); var callbackText string 
		switch callbackData {
		case CallbackConfirmCloseGoal:
			log.Printf("Підтверджено закриття цілі для %d", chatID); err := DeleteUserGoal(chatID, srv, cfg) 
			if err != nil { if strings.Contains(err.Error(), "не знайдено активної цілі") { callbackText = "ℹ️ Активну ціль не знайдено." } else { callbackText = "⚠️ Помилка закриття цілі."; log.Printf("DeleteUserGoal err: %v", err) } } else { callbackText = "✅ Ціль успішно закрито!" }
			originalText := ""; if update.CallbackQuery.Message != nil { originalText = update.CallbackQuery.Message.Text }; editText := tgbotapi.NewEditMessageText(chatID, messageID, originalText+"\n\n"+callbackText); bot.Send(editText)
			keyboard.ShowMainKeyboard(bot, chatID) 
		case CallbackCancelCloseGoal:
			log.Printf("Скасовано закриття цілі для %d", chatID); callbackText = "🚫 Закриття цілі скасовано."
			originalText := ""; if update.CallbackQuery.Message != nil { originalText = update.CallbackQuery.Message.Text }; editText := tgbotapi.NewEditMessageText(chatID, messageID, originalText+"\n\n"+callbackText); bot.Send(editText)
			keyboard.ShowMainKeyboard(bot, chatID)
		default:
			log.Printf("Передача Callback '%s' в goal.HandleCallback", callbackData)
			goal.HandleCallback(bot, update.CallbackQuery, srv, cfg) // Передаємо cfg
		}
		answerCallback := tgbotapi.NewCallback(update.CallbackQuery.ID, ""); if _, err := bot.Request(answerCallback); err != nil { log.Printf("Помилка AnswerCallbackQuery: %v", err) }
		return
	}

	// Обробка звичайних повідомлень
	if update.Message == nil { return }
	chatID := update.Message.Chat.ID; msgText := update.Message.Text; userName := update.Message.From.UserName
	log.Printf("[%s] (%d): %s", userName, chatID, msgText)
	currentState := GetUserState(chatID)

	// --- Перевірка Станів ---
	if currentState == StateAwaitingGoalInput {
		isCommandOrButton := false; switch msgText { case "/start", "🔁 Старт", "/stop", "⛔️ Стоп", "/dayoff", "🏖 Вихідний", "/goal", "🎯 Моя ціль", "/closegoal", "❌ Закрити ціль", "/motivation", "/report", "📊 Прогрес", "/add_investment": isCommandOrButton = true }
		if isCommandOrButton { log.Printf("Користувач %s в '%s', але надіслав '%s'. Скидаємо стан.", userName, currentState, msgText); SetUserState(chatID, StateDefault) } else { log.Printf("Обробка від [%s] як введення ЦІЛІ (стан: %s)", userName, currentState); HandleGoalInput(bot, update.Message, srv, cfg); SetUserState(chatID, StateDefault); keyboard.ShowMainKeyboard(bot, chatID); return }
		currentState = GetUserState(chatID) 
	} else if currentState == StateAwaitingInvestmentInput { 
		isCommandOrButton := false; switch msgText { case "/start", "🔁 Старт", "/stop", "⛔️ Стоп", "/dayoff", "🏖 Вихідний", "/goal", "🎯 Моя ціль", "/closegoal", "❌ Закрити ціль", "/motivation", "/report", "📊 Прогрес", "/add_investment": isCommandOrButton = true }
		if isCommandOrButton { log.Printf("Користувач %s в '%s', але надіслав '%s'. Скидаємо стан.", userName, currentState, msgText); SetUserState(chatID, StateDefault) } else { log.Printf("Обробка від [%s] як введення ІНВЕСТИЦІЇ (стан: %s)", userName, currentState); HandleInvestmentInput(bot, update.Message, srv, cfg); SetUserState(chatID, StateDefault); keyboard.ShowMainKeyboard(bot, chatID); return }
		currentState = GetUserState(chatID) 
	}
	// --- Обробка Команд / Кнопок ---
	switch msgText {
	case "/start", "🔁 Старт":
		commands.StartWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/stop", "⛔️ Стоп":
		commands.StopWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/dayoff", "🏖 Вихідний":
		commands.DayOff(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/goal", "🎯 Моя ціль":
		log.Printf("Обробка команди /goal для ChatID: %d", chatID); currentGoal, exists := GetUserGoal(chatID, srv, cfg) 
		if exists { log.Printf("Знайдено ціль для %d: %+v", chatID, currentGoal); goalInfoText := fmt.Sprintf("📌 Ваша поточна фінансова ціль:\n\n"+"Сума: `%.2f %s`\n"+"Термін: `%d днів`\n"+"Встановлено: `%s`\n\n"+"...", currentGoal.Amount, currentGoal.Currency, currentGoal.Days, currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006")); msg := tgbotapi.NewMessage(chatID, goalInfoText); msg.ParseMode = tgbotapi.ModeMarkdown; if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання інфо про ціль: %v", err) } else { log.Printf("Повідомлення про поточну ціль надіслано для %d", chatID) }; keyboard.ShowMainKeyboard(bot, chatID) 
		} else { log.Printf("Активна ціль для %d не знайдена.", chatID); goal.HandleMyGoalCommand(bot, chatID); SetUserState(chatID, StateAwaitingGoalInput); log.Printf("Стан для %d встановлено в StateAwaitingGoalInput", chatID) }
	case "/closegoal", "❌ Закрити ціль": 
		log.Printf("Обробка команди /closegoal для ChatID: %d", chatID); currentGoal, exists := GetUserGoal(chatID, srv, cfg) 
		if exists { confirmationText := fmt.Sprintf("❓ Ви впевнені?\n\n"+"Сума: `%.2f %s`\n"+"Термін: `%d днів`\n"+"Встановлено: `%s`", currentGoal.Amount, currentGoal.Currency, currentGoal.Days, currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006")); msg := tgbotapi.NewMessage(chatID, confirmationText); msg.ParseMode = tgbotapi.ModeMarkdown; msg.ReplyMarkup = keyboard.CreateConfirmationKeyboard(CallbackConfirmCloseGoal, CallbackCancelCloseGoal); if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання запиту підтвердження: %v", err) }
		} else { log.Printf("Немає цілі для закриття для %d", chatID); msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної цілі."); if _, err := bot.Send(msg); err != nil { log.Printf("Помилка 'немає цілі': %v", err) }; keyboard.ShowMainKeyboard(bot, chatID) }
	
	// <<< ДОДАНО КЕЙС ДЛЯ НОВОЇ КОМАНДИ >>>
	case "/add_investment":
		log.Printf("Обробка команди /add_investment для ChatID: %d", chatID)
		prompt := "➕ Введіть дані нової інвестиції у форматі:\n`ТИП, НАЗВА, СУМА ВАЛЮТА, ДАТА (РРРР-ММ-ДД)`\n\nПриклад: `Акція, AAPL, 1000 USD, 2024-03-15`"
		msg := tgbotapi.NewMessage(chatID, prompt); msg.ParseMode = tgbotapi.ModeMarkdown
		if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання запиту на введення інвестиції: %v", err)
		} else { SetUserState(chatID, StateAwaitingInvestmentInput); log.Printf("Стан для %d встановлено в StateAwaitingInvestmentInput", chatID) }

	case "/motivation":
		motivationText := motivation.GetRandomMotivation(); msg := tgbotapi.NewMessage(chatID, motivationText); if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання мотивації: %v", err) }
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/report", "📊 Прогрес":
		ReportProgress(bot, update.Message, srv, cfg) 
	default:
		log.Printf("Не розпізнана команда від [%s]: %s.", userName, msgText)
		keyboard.ShowMainKeyboard(bot, chatID)
	}
}
