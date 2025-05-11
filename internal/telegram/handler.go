package telegram

import (
	"fmt"
	"log"
	"sort"
	"strings"
	// "time" // Не потрібен тут напряму

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/binance"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

const ( /* ... константи Callback ... */ 
	CallbackConfirmCloseGoal = "confirm_close_goal"
	CallbackCancelCloseGoal  = "cancel_close_goal"
)

func sendAndLog(bot *tgbotapi.BotAPI, c tgbotapi.Chattable, commandName string, chatID int64) {
	if _, err := bot.Send(c); err != nil {
		log.Printf("ПОМИЛКА bot.Send для команди '%s', ChatID %d: %v", commandName, chatID, err)
	} else {
		log.Printf("Повідомлення для команди '%s' успішно надіслано ChatID %d.", commandName, chatID)
	}
}

func requestAndLog(bot *tgbotapi.BotAPI, c tgbotapi.RequestConfig, commandName string, chatID int64) {
	if _, err := bot.Request(c); err != nil {
		log.Printf("ПОМИЛКА bot.Request для '%s' (Callback Answer), ChatID %d: %v", commandName, chatID, err)
	} else {
		log.Printf("Відповідь на Callback для '%s' успішно надіслано ChatID %d.", commandName, chatID)
	}
}


func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID; messageID := update.CallbackQuery.Message.MessageID; userName := update.CallbackQuery.From.UserName; callbackData := update.CallbackQuery.Data
		log.Printf("Callback від [%s](%d): Data=%s, MsgID=%d", userName, chatID, callbackData, messageID); var callbackText string
		originalMessageText := ""; if update.CallbackQuery.Message != nil { originalMessageText = update.CallbackQuery.Message.Text }
		
		switch callbackData {
		case CallbackConfirmCloseGoal:
			log.Printf("Підтверджено закриття цілі для %d", chatID); err := DeleteUserGoal(chatID, srv, cfg)
			if err != nil { if strings.Contains(err.Error(), "не знайдено активної цілі") { callbackText = "ℹ️ Активну ціль не знайдено." } else { callbackText = "⚠️ Помилка закриття цілі."; log.Printf("DeleteUserGoal err: %v", err) } } else { callbackText = "✅ Ціль успішно закрито!" }
			editText := tgbotapi.NewEditMessageText(chatID, messageID, originalMessageText+"\n\n"+callbackText); editText.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, editText, "confirm_close_goal_edit", chatID)
			keyboard.ShowMainKeyboard(bot, chatID)
		case CallbackCancelCloseGoal:
			log.Printf("Скасовано закриття цілі для %d", chatID); callbackText = "🚫 Закриття цілі скасовано."
			editText := tgbotapi.NewEditMessageText(chatID, messageID, originalMessageText+"\n\n"+callbackText); editText.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, editText, "cancel_close_goal_edit", chatID)
			keyboard.ShowMainKeyboard(bot, chatID)
		default:
			log.Printf("Передача Callback '%s' в goal.HandleCallback", callbackData)
			goal.HandleCallback(bot, update.CallbackQuery, srv, cfg) 
		}
		answerCallback := tgbotapi.NewCallback(update.CallbackQuery.ID, callbackText); // Додамо текст у відповідь на callback
		requestAndLog(bot, answerCallback, "answer_callback", chatID)
		return
	}

	if update.Message == nil { return }
	chatID := update.Message.Chat.ID; msgText := update.Message.Text; userName := update.Message.From.UserName
	log.Printf("[%s] (%d): %s", userName, chatID, msgText)
	currentState := GetUserState(chatID)

	if currentState == StateAwaitingGoalInput { /* ... (код без змін, але всі Send мають бути через sendAndLog) ... */ } else if currentState == StateAwaitingInvestmentInput { /* ... (код без змін, але всі Send мають бути через sendAndLog) ... */ }
	
	switch msgText {
	case "/start", "🔁 Старт": commands.StartWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/stop", "⛔️ Стоп": commands.StopWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/dayoff", "🏖 Вихідний": commands.DayOff(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/goal", "🎯 Моя ціль": 
		log.Printf("Обробка /goal для %d", chatID); currentGoal, exists := GetUserGoal(chatID, srv, cfg) 
		if exists { 
			goalInfoText := fmt.Sprintf("📌 Ваша поточна ціль:\n\nСума: `%.2f %s`\nТермін: `0 днів` (Ціль на %s %d)\nВстановлено: `%s`\n\nЩоб встановити нову ціль...", currentGoal.Amount, currentGoal.Currency, monthNameUkrainian(currentGoal.SetDate.In(sheets.KyivLocation).Month()), currentGoal.SetDate.In(sheets.KyivLocation).Year(), currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006")); 
			msg := tgbotapi.NewMessage(chatID, goalInfoText); msg.ParseMode = tgbotapi.ModeMarkdown; 
			sendAndLog(bot, msg, "view_goal_exists", chatID)
			keyboard.ShowMainKeyboard(bot, chatID) 
		} else { 
			log.Printf("Активна ціль для %d не знайдена.", chatID); 
			goal.HandleMyGoalCommand(bot, chatID); // Ця функція сама надсилає повідомлення
			SetUserState(chatID, StateAwaitingGoalInput); 
			log.Printf("Стан %d -> awaiting_goal", chatID) 
		}
	case "/closegoal", "❌ Закрити ціль": 
		log.Printf("Обробка /closegoal для %d", chatID); currentGoal, exists := GetUserGoal(chatID, srv, cfg) 
		if exists { 
			confirmationText := fmt.Sprintf("❓ Впевнені?\nСума: `%.2f %s`\nВстановлено: `%s`", currentGoal.Amount, currentGoal.Currency, currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006")); 
			msg := tgbotapi.NewMessage(chatID, confirmationText); msg.ParseMode = tgbotapi.ModeMarkdown; 
			msg.ReplyMarkup = keyboard.CreateConfirmationKeyboard(CallbackConfirmCloseGoal, CallbackCancelCloseGoal); 
			sendAndLog(bot, msg, "close_goal_confirm_prompt", chatID)
		} else { 
			msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної цілі."); 
			sendAndLog(bot, msg, "close_goal_no_active", chatID)
			keyboard.ShowMainKeyboard(bot, chatID) 
		}
	case "/add_investment": 
		log.Printf("Обробка /add_investment для %d", chatID); 
		prompt := "➕ Введіть інвестицію:\n`ТИП, НАЗВА, СУМА ВАЛЮТА, ДАТА (РРРР-ММ-ДД)`\nПриклад: `Акція, AAPL, 1000 USD, 2024-03-15`"; 
		msg := tgbotapi.NewMessage(chatID, prompt); msg.ParseMode = tgbotapi.ModeMarkdown; 
		sendAndLog(bot, msg, "add_investment_prompt", chatID)
		SetUserState(chatID, StateAwaitingInvestmentInput); log.Printf("Стан %d -> awaiting_investment", chatID)
	
	case "/funding", "💹 Funding Rates": 
		log.Printf("Обробка команди /funding для ChatID: %d", chatID); 
		loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Завантажую..."); 
		sentMsgObj, errSendLoad := bot.Send(loadingMsg);
		if errSendLoad != nil { log.Printf("ПОМИЛКА bot.Send (loadingMsg) для /funding: %v", errSendLoad)}

		var fundingReportText string // ... (решта логіки /funding як у відповіді #233) ...
		// В кінці, для оновлення або надсилання нового повідомлення:
		if sentMsgObj.MessageID != 0 && errSendLoad == nil { // Лише якщо початкове повідомлення було надіслано
			editText := tgbotapi.NewEditMessageText(chatID, sentMsgObj.MessageID, fundingReportText); editText.ParseMode = tgbotapi.ModeMarkdown; 
			sendAndLog(bot, editText, "funding_report_edit", chatID)
		} else { 
			finalMsg := tgbotapi.NewMessage(chatID, fundingReportText); finalMsg.ParseMode = tgbotapi.ModeMarkdown; 
			sendAndLog(bot, finalMsg, "funding_report_new", chatID)
		}
		keyboard.ShowMainKeyboard(bot, chatID)

	case "/motivation": 
		motivationText := motivation.GetRandomMotivation(); 
		msg := tgbotapi.NewMessage(chatID, motivationText); 
		sendAndLog(bot, msg, "motivation", chatID)
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/report", "📊 Прогрес":
		ReportProgress(bot, update.Message, srv, cfg) // ReportProgress сам надсилає повідомлення
	default:
		log.Printf("Не розпізнана команда: [%s]: %s.", userName, msgText)
		keyboard.ShowMainKeyboard(bot, chatID) // ShowMainKeyboard також надсилає повідомлення
	}
}

// Важливо: Переконайтеся, що функції HandleGoalInput, commands.StartWork, commands.StopWork, commands.DayOff, ReportProgress
// та goal.HandleMyGoalCommand також використовують sendAndLog для всіх своїх bot.Send, якщо ви хочете уніфікувати логування.
// Я тут змінив лише прямі виклики bot.Send в HandleUpdate.
