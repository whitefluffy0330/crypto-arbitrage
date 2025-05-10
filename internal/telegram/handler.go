package telegram

import (
	"fmt"
	"log"
	"strings"
	// "time" // ВИДАЛЕНО, оскільки не використовується напряму тут

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/binance"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" // Підпакет goal - ВИКОРИСТОВУЄТЬСЯ
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation" // ВИКОРИСТОВУЄТЬСЯ
	gsheets "google.golang.org/api/sheets/v4"
)

const (
	CallbackConfirmCloseGoal = "confirm_close_goal"
	CallbackCancelCloseGoal  = "cancel_close_goal"
)

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
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
			goal.HandleCallback(bot, update.CallbackQuery, srv, cfg) // ВИКОРИСТАННЯ підпакета goal
		}
		answerCallback := tgbotapi.NewCallback(update.CallbackQuery.ID, ""); if _, err := bot.Request(answerCallback); err != nil { log.Printf("Помилка AnswerCallbackQuery: %v", err) }
		return
	}

	if update.Message == nil { return }
	chatID := update.Message.Chat.ID; msgText := update.Message.Text; userName := update.Message.From.UserName
	log.Printf("[%s] (%d): %s", userName, chatID, msgText)
	currentState := GetUserState(chatID)

	if currentState == StateAwaitingGoalInput {
		isCommandOrButton := false; switch msgText { case "/start", "🔁 Старт", "/stop", "⛔️ Стоп", "/dayoff", "🏖 Вихідний", "/goal", "🎯 Моя ціль", "/closegoal", "❌ Закрити ціль", "/motivation", "/report", "📊 Прогрес", "/add_investment", "/funding", "💹 Funding Rates": isCommandOrButton = true }
		if isCommandOrButton { log.Printf("Користувач %s в '%s', але надіслав '%s'. Скидаємо стан.", userName, currentState, msgText); SetUserState(chatID, StateDefault)  } else { log.Printf("Обробка від [%s] як введення ЦІЛІ (стан: %s)", userName, currentState); HandleGoalInput(bot, update.Message, srv, cfg); SetUserState(chatID, StateDefault); keyboard.ShowMainKeyboard(bot, chatID); return }
		currentState = GetUserState(chatID) 
	} else if currentState == StateAwaitingInvestmentInput { 
		isCommandOrButton := false; switch msgText { case "/start", "🔁 Старт", "/stop", "⛔️ Стоп", "/dayoff", "🏖 Вихідний", "/goal", "🎯 Моя ціль", "/closegoal", "❌ Закрити ціль", "/motivation", "/report", "📊 Прогрес", "/add_investment", "/funding", "💹 Funding Rates": isCommandOrButton = true }
		if isCommandOrButton { log.Printf("Користувач %s в '%s', але надіслав '%s'. Скидаємо стан.", userName, currentState, msgText); SetUserState(chatID, StateDefault) } else { log.Printf("Обробка від [%s] як введення ІНВЕСТИЦІЇ (стан: %s)", userName, currentState); HandleInvestmentInput(bot, update.Message, srv, cfg); SetUserState(chatID, StateDefault); keyboard.ShowMainKeyboard(bot, chatID); return }
		currentState = GetUserState(chatID) 
	}
	
	switch msgText {
	case "/start", "🔁 Старт": commands.StartWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/stop", "⛔️ Стоп": commands.StopWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/dayoff", "🏖 Вихідний": commands.DayOff(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/goal", "🎯 Моя ціль": 
		log.Printf("Обробка /goal для %d", chatID); currentGoal, exists := GetUserGoal(chatID, srv, cfg) 
		if exists { 
			log.Printf("Знайдено існуючу ціль для %d: %+v", chatID, currentGoal); 
			goalInfoText := fmt.Sprintf("📌 Ваша поточна фінансова ціль:\n\nСума: `%.2f %s`\nТермін: `%d днів`\nВстановлено: `%s`\n\nЩоб встановити нову ціль (вона перезапише поточну), надішліть її деталі у форматі `СУМА [ВАЛЮТА], КІЛЬКІСТЬ_ДНІВ днів` після цього повідомлення, або спочатку закрийте поточну ціль.", currentGoal.Amount, currentGoal.Currency, currentGoal.Days, currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006")); 
			msg := tgbotapi.NewMessage(chatID, goalInfoText); msg.ParseMode = tgbotapi.ModeMarkdown; 
			if _, err := bot.Send(msg); err != nil { log.Printf("Помилка: %v", err) } else { log.Printf("Інфо-ціль %d", chatID) }; 
			keyboard.ShowMainKeyboard(bot, chatID) 
		} else { 
			log.Printf("Активна ціль для %d не знайдена.", chatID); 
			goal.HandleMyGoalCommand(bot, chatID); // ВИКОРИСТАННЯ підпакета goal
			SetUserState(chatID, StateAwaitingGoalInput); 
			log.Printf("Стан %d -> awaiting_goal", chatID) 
		}
	case "/closegoal", "❌ Закрити ціль": 
		log.Printf("Обробка /closegoal для %d", chatID); currentGoal, exists := GetUserGoal(chatID, srv, cfg) 
		if exists { 
			confirmationText := fmt.Sprintf("❓ Ви впевнені?\nСума: `%.2f %s`\nТермін: `%d дн.`\nВстановлено: `%s`", currentGoal.Amount, currentGoal.Currency, currentGoal.Days, currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006")); 
			msg := tgbotapi.NewMessage(chatID, confirmationText); msg.ParseMode = tgbotapi.ModeMarkdown; 
			msg.ReplyMarkup = keyboard.CreateConfirmationKeyboard(CallbackConfirmCloseGoal, CallbackCancelCloseGoal); 
			if _, err := bot.Send(msg); err != nil {log.Printf("Помилка підтвердження: %v", err)} 
		} else { 
			log.Printf("Немає цілі для закриття для %d", chatID); 
			msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної цілі."); if _, err := bot.Send(msg); err != nil {log.Printf("Помилка 'немає цілі': %v", err)}; 
			keyboard.ShowMainKeyboard(bot, chatID) 
		}
	case "/add_investment": 
		log.Printf("Обробка /add_investment для %d", chatID); 
		prompt := "➕ Введіть інвестицію:\n`ТИП, НАЗВА, СУМА ВАЛЮТА, ДАТА (РРРР-ММ-ДД)`\nПриклад: `Акція, AAPL, 1000 USD, 2024-03-15`"; 
		msg := tgbotapi.NewMessage(chatID, prompt); msg.ParseMode = tgbotapi.ModeMarkdown; 
		if _, err := bot.Send(msg); err != nil {log.Printf("Помилка запиту інвестиції: %v", err)} else { SetUserState(chatID, StateAwaitingInvestmentInput); log.Printf("Стан %d -> awaiting_investment", chatID) }
	
	case "/funding", "💹 Funding Rates": 
		log.Printf("Обробка команди /funding для ChatID: %d", chatID); 
		loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Завантажую ставки Binance..."); 
		sentMsg, _ := bot.Send(loadingMsg)
		rates, err := binance.GetFundingRates(); var fundingReportText string
		if err != nil { log.Printf("Помилка funding rates: %v", err); fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати ставки: %v", err)
		} else {
			if len(rates) == 0 { fundingReportText = "Інформація про ставки недоступна." } else {
				var sb strings.Builder; sb.WriteString("📊 **Funding Rates (Binance Futures):**\n\n");
				symbolsToShow := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT", "DOGEUSDT"}; shownCount := 0
				for _, symbol := range symbolsToShow {
					if info, ok := rates[symbol]; ok {
						nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation) 
						sb.WriteString(fmt.Sprintf("`%s`:\n  Rate: `%.4f%%` (Next: %s)\n  Mark: `%.2f`\n\n", info.Symbol, info.LastFundingRate, nextTimeKyiv.Format("15:04"), info.MarkPrice))
						shownCount++
					}
				}
				if shownCount == 0 { sb.WriteString("Не знайдено даних для стандартних символів.") }; fundingReportText = sb.String()
			}
		}
		if sentMsg.MessageID != 0 { editText := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, fundingReportText); editText.ParseMode = tgbotapi.ModeMarkdown; bot.Send(editText) } else { finalMsg := tgbotapi.NewMessage(chatID, fundingReportText); finalMsg.ParseMode = tgbotapi.ModeMarkdown; bot.Send(finalMsg) }
		keyboard.ShowMainKeyboard(bot, chatID)

	case "/motivation": 
		// ВИКОРИСТАННЯ пакета motivation
		motivationText := motivation.GetRandomMotivation(); 
		msg := tgbotapi.NewMessage(chatID, motivationText); if _, err := bot.Send(msg); err != nil { log.Printf("Помилка мотивації: %v", err) }; 
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/report", "📊 Прогрес":
		ReportProgress(bot, update.Message, srv, cfg) 
	default:
		log.Printf("Не розпізнана команда: [%s]: %s.", userName, msgText)
		keyboard.ShowMainKeyboard(bot, chatID)
	}
}
