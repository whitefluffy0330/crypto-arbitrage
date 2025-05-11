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
		chatID := update.CallbackQuery.Message.Chat.ID
		messageID := update.CallbackQuery.Message.MessageID
		userName := update.CallbackQuery.From.UserName
		callbackData := update.CallbackQuery.Data
		log.Printf("Callback від [%s](%d): Data=%s, MsgID=%d", userName, chatID, callbackData, messageID)

		var callbackText string
		originalMessageText := "" // Текст вихідного повідомлення для редагування
		if update.CallbackQuery.Message != nil {
			originalMessageText = update.CallbackQuery.Message.Text
		}

		switch callbackData {
		case CallbackConfirmCloseGoal:
			log.Printf("Підтверджено закриття цілі для %d", chatID)
			err := DeleteUserGoal(chatID, srv, cfg)
			if err != nil {
				if strings.Contains(err.Error(), "не знайдено активної цілі") {
					callbackText = "ℹ️ Активну ціль не знайдено. Можливо, її вже було закрито."
					log.Printf("Спроба закрити неіснуючу/вже закриту ціль для ChatID: %d", chatID)
				} else {
					callbackText = "⚠️ Помилка закриття цілі. Спробуйте пізніше."
					log.Printf("Помилка DeleteUserGoal для ChatID %d: %v", chatID, err)
				}
			} else {
				callbackText = "✅ Ціль успішно закрито!"
			}
			editText := tgbotapi.NewEditMessageText(chatID, messageID, originalMessageText+"\n\n"+callbackText)
			bot.Send(editText)
			keyboard.ShowMainKeyboard(bot, chatID)

		case CallbackCancelCloseGoal:
			log.Printf("Скасовано закриття цілі для %d", chatID)
			callbackText = "🚫 Закриття цілі скасовано."
			editText := tgbotapi.NewEditMessageText(chatID, messageID, originalMessageText+"\n\n"+callbackText)
			bot.Send(editText)
			keyboard.ShowMainKeyboard(bot, chatID)

		default:
			log.Printf("Передача Callback '%s' в goal.HandleCallback", callbackData)
			goal.HandleCallback(bot, update.CallbackQuery, srv, cfg) // ВИКОРИСТАННЯ goal
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

	// --- Перевірка Станів ---
	if currentState == StateAwaitingGoalInput {
		isCommandOrButton := false
		switch msgText { case "/start", "🔁 Старт", "/stop", "⛔️ Стоп", "/dayoff", "🏖 Вихідний", "/goal", "🎯 Моя ціль", "/closegoal", "❌ Закрити ціль", "/motivation", "/report", "📊 Прогрес", "/add_investment", "/funding", "💹 Funding Rates": isCommandOrButton = true }
		if isCommandOrButton { 
			log.Printf("Користувач %s в '%s', але надіслав '%s'. Скидаємо стан.", userName, currentState, msgText)
			SetUserState(chatID, StateDefault) 
		} else { 
			log.Printf("Обробка від [%s] як введення ЦІЛІ (стан: %s)", userName, currentState)
			HandleGoalInput(bot, update.Message, srv, cfg) 
			SetUserState(chatID, StateDefault)
			keyboard.ShowMainKeyboard(bot, chatID)
			return 
		}
		currentState = GetUserState(chatID) 
	} else if currentState == StateAwaitingInvestmentInput { 
		isCommandOrButton := false
		switch msgText { case "/start", "🔁 Старт", "/stop", "⛔️ Стоп", "/dayoff", "🏖 Вихідний", "/goal", "🎯 Моя ціль", "/closegoal", "❌ Закрити ціль", "/motivation", "/report", "📊 Прогрес", "/add_investment", "/funding", "💹 Funding Rates": isCommandOrButton = true }
		if isCommandOrButton { 
			log.Printf("Користувач %s в '%s', але надіслав '%s'. Скидаємо стан.", userName, currentState, msgText)
			SetUserState(chatID, StateDefault) 
		} else { 
			log.Printf("Обробка від [%s] як введення ІНВЕСТИЦІЇ (стан: %s)", userName, currentState)
			HandleInvestmentInput(bot, update.Message, srv, cfg) 
			SetUserState(chatID, StateDefault)
			keyboard.ShowMainKeyboard(bot, chatID)
			return 
		}
		currentState = GetUserState(chatID) 
	}

	// --- Обробка Команд / Кнопок ---
	switch msgText {
	case "/start", "🔁 Старт": 
		commands.StartWork(bot, update.Message, srv, cfg)
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/stop", "⛔️ Стоп": 
		commands.StopWork(bot, update.Message, srv, cfg)
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/dayoff", "🏖 Вихідний": 
		commands.DayOff(bot, update.Message, srv, cfg)
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/goal", "🎯 Моя ціль": 
		log.Printf("Обробка команди /goal для ChatID: %d", chatID)
		currentGoal, exists := GetUserGoal(chatID, srv, cfg) 
		if exists { 
			log.Printf("Знайдено існуючу ціль для %d: %+v", chatID, currentGoal)
			goalInfoText := fmt.Sprintf("📌 Ваша поточна фінансова ціль:\n\nСума: `%.2f %s`\nТермін: `0 днів` (Ціль на %s %d)\nВстановлено: `%s`\n\nЩоб встановити нову ціль на поточний місяць (вона перезапише поточну), надішліть її деталі у форматі `СУМА [ВАЛЮТА]`.", 
			    currentGoal.Amount, currentGoal.Currency, 
			    monthNameUkrainian(currentGoal.SetDate.In(sheets.KyivLocation).Month()), currentGoal.SetDate.In(sheets.KyivLocation).Year(), // Показуємо місяць та рік цілі
			    currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006"))
			msg := tgbotapi.NewMessage(chatID, goalInfoText); msg.ParseMode = tgbotapi.ModeMarkdown 
			if _, err := bot.Send(msg); err != nil { log.Printf("Помилка надсилання інфо про ціль: %v", err) } else { log.Printf("Повідомлення про поточну ціль надіслано для %d", chatID) }
			keyboard.ShowMainKeyboard(bot, chatID) 
		} else { 
			log.Printf("Активна ціль для %d не знайдена.", chatID)
			goal.HandleMyGoalCommand(bot, chatID) // ВИКОРИСТАННЯ підпакета goal
			SetUserState(chatID, StateAwaitingGoalInput)
			log.Printf("Стан для %d встановлено в StateAwaitingGoalInput", chatID) 
		}
	case "/closegoal", "❌ Закрити ціль": 
		log.Printf("Обробка команди /closegoal для ChatID: %d", chatID)
		currentGoal, exists := GetUserGoal(chatID, srv, cfg) 
		if exists { 
			confirmationText := fmt.Sprintf("❓ Ви впевнені, що хочете закрити поточну ціль на %s %d?\n\nСума: `%.2f %s`\nВстановлено: `%s`", 
			    monthNameUkrainian(currentGoal.SetDate.In(sheets.KyivLocation).Month()), currentGoal.SetDate.In(sheets.KyivLocation).Year(), // Показуємо місяць та рік цілі
			    currentGoal.Amount, currentGoal.Currency, currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006"))
			msg := tgbotapi.NewMessage(chatID, confirmationText); msg.ParseMode = tgbotapi.ModeMarkdown 
			msg.ReplyMarkup = keyboard.CreateConfirmationKeyboard(CallbackConfirmCloseGoal, CallbackCancelCloseGoal) 
			if _, err := bot.Send(msg); err != nil {log.Printf("Помилка надсилання запиту підтвердження: %v", err)} 
		} else { 
			log.Printf("Немає цілі для закриття для %d", chatID) 
			msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної цілі."); if _, err := bot.Send(msg); err != nil {log.Printf("Помилка 'немає цілі': %v", err)}; 
			keyboard.ShowMainKeyboard(bot, chatID) 
		}
	case "/add_investment": 
		log.Printf("Обробка /add_investment для %d", chatID) 
		prompt := "➕ Введіть інвестицію:\n`ТИП, НАЗВА, СУМА ВАЛЮТА, ДАТА (РРРР-ММ-ДД)`\nПриклад: `Акція, AAPL, 1000 USD, 2024-03-15`"; 
		msg := tgbotapi.NewMessage(chatID, prompt); msg.ParseMode = tgbotapi.ModeMarkdown; 
		if _, err := bot.Send(msg); err != nil {log.Printf("Помилка запиту інвестиції: %v", err)} else { SetUserState(chatID, StateAwaitingInvestmentInput); log.Printf("Стан %d -> awaiting_investment", chatID) }

	case "/funding", "💹 Funding Rates": 
		log.Printf("Обробка команди /funding для ChatID: %d", chatID); 
		loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Завантажую топ-монети з CoinGecko та ставки з Binance..."); 
		sentMsg, _ := bot.Send(loadingMsg)
		var fundingReportText string
		topCoins, errCoinGecko := coingecko.GetTopMarketCapCoins(20, "usd")
		if errCoinGecko != nil { log.Printf("Помилка CoinGecko: %v", errCoinGecko); fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати топ-монети: %v", errCoinGecko)
		} else if len(topCoins) == 0 { fundingReportText = "Не знайдено топ-монет на CoinGecko."
		} else {
			var targetBinanceSymbols []string; for _, coin := range topCoins { targetBinanceSymbols = append(targetBinanceSymbols, strings.ToUpper(coin.Symbol) + "USDT") }
			log.Printf("Сформовано %d цільових символів: %v", len(targetBinanceSymbols), targetBinanceSymbols)
			allRatesMap, errBinance := binance.GetFundingRates()
			if errBinance != nil { log.Printf("Помилка Binance: %v", errBinance); fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати ставки Binance: %v", errBinance)
			} else if len(allRatesMap) == 0 { fundingReportText = "Ставки Binance недоступні."
			} else {
				var relevantRatesSlice []binance.FundingInfo; for _, s := range targetBinanceSymbols { if r, ok := allRatesMap[s]; ok { relevantRatesSlice = append(relevantRatesSlice, r) } }
				log.Printf("Знайдено %d релевантних ставок Binance.", len(relevantRatesSlice))
				if len(relevantRatesSlice) == 0 { fundingReportText = "Не знайдено ставок для топ-монет на Binance."
				} else {
					sort.SliceStable(relevantRatesSlice, func(i, j int) bool { if relevantRatesSlice[i].LastFundingRate > 0 && relevantRatesSlice[j].LastFundingRate <= 0 { return true }; if relevantRatesSlice[i].LastFundingRate <= 0 && relevantRatesSlice[j].LastFundingRate > 0 { return false }; if relevantRatesSlice[i].LastFundingRate > 0 && relevantRatesSlice[j].LastFundingRate > 0 { return relevantRatesSlice[i].LastFundingRate > relevantRatesSlice[j].LastFundingRate }; return relevantRatesSlice[i].LastFundingRate < relevantRatesSlice[j].LastFundingRate })
					var sb strings.Builder; sb.WriteString("📊 **Funding Rates (Binance Futures) для Топ-Монет:**\n_(оцінка для $100 за 8 год.)_\n\n"); limit := 7; posCount := 0; negCount := 0
					sb.WriteString("📈 **Найвищі Позитивні (вигідно Short):**\n"); foundPos := false
					for _, info := range relevantRatesSlice { if posCount >= limit { break }; if info.LastFundingRate > 0.001 { profit := info.LastFundingRate; sb.WriteString(fmt.Sprintf("`%s`: `%.4f%%` (+$%.2f) (Next: %s)\n", info.Symbol, info.LastFundingRate, profit, info.NextFundingTime.In(sheets.KyivLocation).Format("15:04"))); posCount++; foundPos = true } }
					if !foundPos { sb.WriteString("_Немає значних позитивних ставок._\n") }; sb.WriteString("\n")
					sb.WriteString("📉 **Найбільш Негативні (вигідно Long):**\n"); foundNeg := false
					for i := len(relevantRatesSlice) - 1; i >= 0; i-- { if negCount >= limit { break }; info := relevantRatesSlice[i]; if info.LastFundingRate < -0.001 { payout := -info.LastFundingRate; sb.WriteString(fmt.Sprintf("`%s`: `%.4f%%` (+$%.2f) (Next: %s)\n", info.Symbol, info.LastFundingRate, payout, info.NextFundingTime.In(sheets.KyivLocation).Format("15:04"))); negCount++; foundNeg = true } }
					if !foundNeg { sb.WriteString("_Немає значних негативних ставок._\n") }; fundingReportText = sb.String()
				}
			}
		}
		if sentMsg.MessageID != 0 { editText := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, fundingReportText); editText.ParseMode = tgbotapi.ModeMarkdown; bot.Send(editText) } else { finalMsg := tgbotapi.NewMessage(chatID, fundingReportText); finalMsg.ParseMode = tgbotapi.ModeMarkdown; bot.Send(finalMsg) }
		keyboard.ShowMainKeyboard(bot, chatID)

	case "/motivation": 
		motivationText := motivation.GetRandomMotivation(); // ВИКОРИСТАННЯ motivation
		msg := tgbotapi.NewMessage(chatID, motivationText); if _, err := bot.Send(msg); err != nil { log.Printf("Помилка мотивації: %v", err) }; 
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/report", "📊 Прогрес":
		ReportProgress(bot, update.Message, srv, cfg) 
	default:
		log.Printf("Не розпізнана команда: [%s]: %s.", userName, msgText)
		keyboard.ShowMainKeyboard(bot, chatID)
	}
}
