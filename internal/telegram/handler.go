package telegram

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/binance"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/bitget" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/bybit" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/mexc" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/okx" 
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // KyivLocation тепер в telegram.KyivLocation
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" // Логіка цілей тепер частково в telegram.go
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	gsheets "google.golang.org/api/sheets/v4"
)

const (
	CallbackConfirmCloseGoal = "confirm_close_goal" 
	CallbackCancelCloseGoal  = "cancel_close_goal"
	MaxTelegramMessageSize   = 4096 
)

func sendAndLog(bot *tgbotapi.BotAPI, c tgbotapi.Chattable, commandName string, chatID int64) {
	if _, err := bot.Send(c); err != nil {
		log.Printf("ПОМИЛКА надсилання (%s) для %d: %v", commandName, chatID, err)
	}
}
func requestAndLog(bot *tgbotapi.BotAPI, c tgbotapi.CallbackConfig, commandName string, chatID int64) {
	if _, err := bot.Request(c); err != nil {
		log.Printf("ПОМИЛКА Request (%s) для %d: %v", commandName, chatID, err)
	}
}

func handleFundingExchangeSelection(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, chatID int64, exchangeName, exchangeCallbackPrefix string) {
	log.Printf("Обробка запиту фандингу для біржі: %s (ChatID: %d)", exchangeName, chatID)
	answerCallback := tgbotapi.NewCallback(query.ID, fmt.Sprintf("Завантажую ставки з %s...", exchangeName))
	requestAndLog(bot, answerCallback, "funding_exchange_ack", chatID)

	loadingMsgText := fmt.Sprintf("⏳ Завантажую ставки з %s...", exchangeName)
	var originalMessageID int 
	if query.Message != nil {
		originalMessageID = query.Message.MessageID
		editMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, loadingMsgText)
		editMsg.ReplyMarkup = nil 
		if _, err := bot.Send(editMsg); err != nil {
			log.Printf("Помилка редагування повідомлення (ID: %d) на 'Завантажую...' для %s: %v. Спробую надіслати нове.", originalMessageID, exchangeName, err)
			newMsg := tgbotapi.NewMessage(chatID, loadingMsgText)
			sentLoadingMsg, errSend := bot.Send(newMsg)
			if errSend != nil {
				log.Printf("Помилка надсилання нового повідомлення 'Завантажую...' для %s: %v", exchangeName, errSend)
				errorText := fmt.Sprintf("Не вдалося ініціювати запит для %s.", exchangeName)
				errMsg := tgbotapi.NewMessage(chatID, errorText)
				sendAndLog(bot, errMsg, "funding_init_error", chatID)
				return
			}
			originalMessageID = sentLoadingMsg.MessageID 
		}
	} else {
		log.Printf("ПОПЕРЕДЖЕННЯ: query.Message is nil для callback %s, ChatID: %d. Надсилаю нове повідомлення 'Завантажую...'", exchangeCallbackPrefix, chatID)
		newMsg := tgbotapi.NewMessage(chatID, loadingMsgText)
		sentLoadingMsg, errSend := bot.Send(newMsg)
		if errSend != nil {
			log.Printf("Помилка надсилання нового повідомлення 'Завантажую...' (query.Message is nil) для %s: %v", exchangeName, errSend)
			return
		}
		originalMessageID = sentLoadingMsg.MessageID
	}

	var rates []exchanges.UnifiedFundingRateInfo
	var err error
	switch exchangeCallbackPrefix {
	case keyboard.CallbackFundingBinance: rates, err = binance.GetFundingRates()
	case keyboard.CallbackFundingBybit: rates, err = bybit.GetFundingRates()
	case keyboard.CallbackFundingOKX: rates, err = okx.GetFundingRates()
	case keyboard.CallbackFundingMEXC: rates, err = mexc.GetFundingRates()
	case keyboard.CallbackFundingBitget: rates, err = bitget.GetFundingRates()
	default:
		log.Printf("Невідомий callback для фандингу: %s", exchangeCallbackPrefix)
		errorText := fmt.Sprintf("Помилка: невідома біржа для запиту (%s).", exchangeName)
		if originalMessageID != 0 {
			finalEditMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, errorText)
			sendAndLog(bot, finalEditMsg, "funding_unknown_exchange_edit", chatID)
		} else {
			sendAndLog(bot, tgbotapi.NewMessage(chatID, errorText), "funding_unknown_exchange_new", chatID)
		}
		keyboard.ShowMainKeyboard(bot, chatID); return
	}

	currentFundingThreshold := GetUserFundingThreshold(chatID) // Використовуємо функцію з telegram.go
	var reportText string
	if err != nil {
		log.Printf("Помилка отримання даних з %s для /funding: %v", exchangeName, err)
		reportText = fmt.Sprintf("⚠️ %s: не вдалося завантажити дані.\nПомилка: %v", exchangeName, err)
	} else if len(rates) == 0 {
		reportText = fmt.Sprintf("ℹ️ %s: дані про ставки фінансування порожні або не знайдено відповідних пар (поріг: `%.4f%%`).", exchangeName, currentFundingThreshold*100) 
	} else {
		sort.SliceStable(rates, func(i, j int) bool {
			rateI := rates[i].LastFundingRate; rateJ := rates[j].LastFundingRate
			if rateI > 0 && rateJ > 0 { return rateI > rateJ }
			if rateI < 0 && rateJ < 0 { return rateI < rateJ }
			return rateI > rateJ
		})
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📊 **Funding Rates (%s):**\n", exchangeName))
		sb.WriteString(fmt.Sprintf("_Поточний поріг відображення: `%.4f%%`._\n", currentFundingThreshold*100)) 
		sb.WriteString("_Ставки ... не враховують торгові комісії._\n\n")
		limit := 5; posCount := 0; negCount := 0
		sb.WriteString("📈 **Найвищі Позитивні Ставки (Long платить Short):**\n"); sb.WriteString("_Для цих пар ..._\n"); sb.WriteString("------------------------------\n"); foundPos := false
		for _, info := range rates {
			if posCount >= limit { break }
			if info.LastFundingRate > currentFundingThreshold*100 { 
				profitPer100 := 100 * (info.LastFundingRate / 100.0)
				var nextFundingDisplay string
				if info.NextFundingTime.IsZero() || info.NextFundingTime.Unix() <= 0 { nextFundingDisplay = "N/A"
				} else {
					nextTimeKyiv := info.NextFundingTime.In(KyivLocation) // Використовуємо KyivLocation з telegram.go
					durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv)) 
					nextFundingDisplay = fmt.Sprintf("%s (через %s)", nextTimeKyiv.Format("15:04 (02.01)"), durationToNext)
				}
				sb.WriteString(fmt.Sprintf("`%s` (%s, Mark: `$%.2f`)\n  Ставка: `+%.4f%%`\n  Прогноз доходу на $100 Short ...: `+$%.2f`\n  Наступна: %s\n",
					info.Symbol, info.Exchange, info.MarkPrice, info.LastFundingRate, profitPer100, nextFundingDisplay)); sb.WriteString("------------------------------\n"); posCount++; foundPos = true
			}
		}
		if !foundPos { sb.WriteString(fmt.Sprintf("_Немає позитивних ставок вище `%.4f%%`._\n", currentFundingThreshold*100)) } 
		sb.WriteString("\n")
		sb.WriteString("📉 **Найбільш Негативні Ставки (Short платить Long):**\n"); sb.WriteString("_Для цих пар ..._\n"); sb.WriteString("------------------------------\n"); foundNeg := false
		tempNegRates := []exchanges.UnifiedFundingRateInfo{}
		for _, info := range rates { if info.LastFundingRate < -currentFundingThreshold*100 { tempNegRates = append(tempNegRates, info) } } 
		sort.SliceStable(tempNegRates, func(i, j int) bool { return tempNegRates[i].LastFundingRate < tempNegRates[j].LastFundingRate })
		for _, info := range tempNegRates {
			if negCount >= limit { break }
			payoutPer100 := 100 * (-info.LastFundingRate / 100.0)
			var nextFundingDisplay string
			if info.NextFundingTime.IsZero() || info.NextFundingTime.Unix() <= 0 { nextFundingDisplay = "N/A"
			} else {
				nextTimeKyiv := info.NextFundingTime.In(KyivLocation) // Використовуємо KyivLocation з telegram.go
				durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv)) 
				nextFundingDisplay = fmt.Sprintf("%s (через %s)", nextTimeKyiv.Format("15:04 (02.01)"), durationToNext)
			}
			sb.WriteString(fmt.Sprintf("`%s` (%s, Mark: `$%.2f`)\n  Ставка: `%.4f%%`\n  Прогноз доходу на $100 Long ...: `+$%.2f`\n  Наступна: %s\n",
				info.Symbol, info.Exchange, info.MarkPrice, info.LastFundingRate, payoutPer100, nextFundingDisplay)); sb.WriteString("------------------------------\n"); negCount++; foundNeg = true
		}
		if !foundNeg { sb.WriteString(fmt.Sprintf("_Немає негативних ставок нижче `-%.4f%%`._\n", currentFundingThreshold*100)) } 
		reportText = sb.String()
	}
	var finalMessageText = reportText
	if len(finalMessageText) > MaxTelegramMessageSize {
		log.Printf("Повідомлення для фандингу %s занадто довге (%d). Обрізаємо.", exchangeName, len(finalMessageText))
		finalMessageText = finalMessageText[:MaxTelegramMessageSize-30] + "\n... (повідомлення обрізано)"
	}
	if originalMessageID != 0 {
		finalEditMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, finalMessageText)
		finalEditMsg.ParseMode = tgbotapi.ModeMarkdown; finalEditMsg.ReplyMarkup = nil 
		if _, err := bot.Send(finalEditMsg); err != nil {
			log.Printf("Помилка фінального редагування для %s (ID: %d): %v. Спроба надіслати нове.", exchangeName, originalMessageID, err)
			newFinalMsg := tgbotapi.NewMessage(chatID, finalMessageText); newFinalMsg.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, newFinalMsg, "funding_report_new_after_edit_fail", chatID)
		}
	} else { 
		log.Printf("originalMessageID is 0 для фандингу %s, надсилаю звіт новим повідомленням.", exchangeName)
		newFinalMsg := tgbotapi.NewMessage(chatID, finalMessageText); newFinalMsg.ParseMode = tgbotapi.ModeMarkdown
		sendAndLog(bot, newFinalMsg, "funding_report_new_no_orig_msgid", chatID)
	}
	keyboard.ShowMainKeyboard(bot, chatID)
}

// HandleUpdate - основний обробник оновлень
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		userName := update.CallbackQuery.From.UserName
		callbackData := update.CallbackQuery.Data
		var msgIDForLog int
		if update.CallbackQuery.Message != nil { msgIDForLog = update.CallbackQuery.Message.MessageID
		} else { log.Printf("ПОПЕРЕДЖЕННЯ: update.CallbackQuery.Message is nil для ChatID %d, Data: %s", chatID, callbackData) }
		log.Printf("Callback від [%s](%d): Data=%s, OrigMsgID=%d", userName, chatID, callbackData, msgIDForLog)
		
		switch callbackData {
		case keyboard.CallbackFundingBinance: handleFundingExchangeSelection(bot, update.CallbackQuery, chatID, "Binance", keyboard.CallbackFundingBinance); return 
		case keyboard.CallbackFundingBybit: handleFundingExchangeSelection(bot, update.CallbackQuery, chatID, "Bybit", keyboard.CallbackFundingBybit); return
		case keyboard.CallbackFundingOKX: handleFundingExchangeSelection(bot, update.CallbackQuery, chatID, "OKX", keyboard.CallbackFundingOKX); return
		case keyboard.CallbackFundingMEXC: handleFundingExchangeSelection(bot, update.CallbackQuery, chatID, "MEXC", keyboard.CallbackFundingMEXC); return
		case keyboard.CallbackFundingBitget: handleFundingExchangeSelection(bot, update.CallbackQuery, chatID, "Bitget", keyboard.CallbackFundingBitget); return
		}

		var callbackResponseText string; originalMessageText := ""; var originalMsgID int 
		if update.CallbackQuery.Message != nil { originalMessageText = update.CallbackQuery.Message.Text; originalMsgID = update.CallbackQuery.Message.MessageID }
		switch callbackData {
		case CallbackConfirmCloseGoal:
			log.Printf("Підтверджено закриття цілі для %d", chatID)
			err := DeleteUserGoal(chatID, srv, cfg) // Використовуємо DeleteUserGoal з telegram.go
			if err != nil {
				if strings.Contains(err.Error(), "не знайдено активної цілі") { callbackResponseText = "ℹ️ Активну ціль не знайдено."
				} else { callbackResponseText = "⚠️ Помилка закриття цілі."; log.Printf("DeleteUserGoal err: %v", err) }
			} else { callbackResponseText = "✅ Ціль успішно закрито!" }
			if originalMsgID != 0 {
				editText := tgbotapi.NewEditMessageText(chatID, originalMsgID, originalMessageText+"\n\n"+callbackResponseText)
				editText.ParseMode = tgbotapi.ModeMarkdown; sendAndLog(bot, editText, "confirm_close_goal_edit", chatID)
			} else { sendAndLog(bot, tgbotapi.NewMessage(chatID, callbackResponseText), "confirm_close_goal_new", chatID) }
			keyboard.ShowMainKeyboard(bot, chatID) 
		case CallbackCancelCloseGoal:
			log.Printf("Скасовано закриття цілі для %d", chatID)
			callbackResponseText = "🚫 Закриття цілі скасовано."
			if originalMsgID != 0 {
				editText := tgbotapi.NewEditMessageText(chatID, originalMsgID, originalMessageText+"\n\n"+callbackResponseText)
				editText.ParseMode = tgbotapi.ModeMarkdown; sendAndLog(bot, editText, "cancel_close_goal_edit", chatID)
			} else { sendAndLog(bot, tgbotapi.NewMessage(chatID, callbackResponseText), "cancel_close_goal_new", chatID) }
			keyboard.ShowMainKeyboard(bot, chatID)
		default:
			log.Printf("Передача Callback '%s' в goal.HandleCallback або невідомий callback", callbackData)
			// Припускаємо, що goal.HandleCallback існує в goal.go
			// Якщо ні, це викличе помилку компіляції "undefined: goal.HandleCallback"
			// Якщо HandleGoalCallback визначена в цьому ж пакеті (telegram), то просто HandleGoalCallback(...)
			// Згідно з вашим файлом goal.go, там є HandleCallback, який може бути HandleGoalCallback
			// Якщо у вас є пакет goal, то має бути goal.HandleCallback
			// Або якщо це функція HandleGoalCallback з файлу goal.go (який є частиною пакету telegram):
			// HandleGoalCallback(bot, update.CallbackQuery, srv, cfg) // Потрібно уточнити
			
			// Тимчасово просто відповідаємо
			answerCallbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "Обробка callback...")
			requestAndLog(bot, answerCallbackCfg, "default_callback_answer", chatID)
			return 
		}
		if callbackResponseText != "" { 
			answerCallbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, callbackResponseText)
			requestAndLog(bot, answerCallbackCfg, "answer_callback_other", chatID)
		}
		return
	}

	if update.Message == nil { return }
	chatID := update.Message.Chat.ID; msgText := update.Message.Text; userName := update.Message.From.UserName
	log.Printf("[%s] (%d): %s", userName, chatID, msgText)
	currentState := GetUserState(chatID) 
	log.Printf("Поточний стан для ChatID %d: '%s'", chatID, currentState)
	isCommandOrButton := strings.HasPrefix(msgText, "/") || msgText == keyboard.BtnWorkStart || msgText == keyboard.BtnWorkStop || msgText == keyboard.BtnWorkDayOff ||
		msgText == keyboard.BtnMyGoal || msgText == keyboard.BtnCloseGoal || msgText == keyboard.BtnAddInvestment ||
		msgText == keyboard.BtnSetFundingThreshold || msgText == keyboard.BtnSpreads || msgText == keyboard.BtnFundingRates ||
		msgText == keyboard.BtnProgress
	if currentState != StateDefault && isCommandOrButton && 
	   !(currentState == StateAwaitingFundingThreshold && (strings.HasPrefix(msgText, "/set_funding_threshold") || msgText == keyboard.BtnSetFundingThreshold)) { 
		log.Printf("Користувач %s в '%s', але надіслав команду/кнопку '%s'. Скидаємо стан.", userName, currentState, msgText)
		SetUserState(chatID, StateDefault); currentState = StateDefault 
	}

	switch currentState {
	case StateAwaitingGoalInput: HandleGoalInput(bot, update.Message, srv, cfg); SetUserState(chatID, StateDefault); keyboard.ShowMainKeyboard(bot, chatID); return
	case StateAwaitingInvestmentInput: HandleInvestmentInput(bot, update.Message, srv, cfg); SetUserState(chatID, StateDefault); keyboard.ShowMainKeyboard(bot, chatID); return
	case StateAwaitingFundingThreshold:
		thresholdStr := strings.TrimSpace(update.Message.Text); threshold, err := strconv.ParseFloat(thresholdStr, 64)
		var responseText string
		if err != nil { responseText = fmt.Sprintf("⚠️ Неправильний формат числа: `%s`. Введіть, напр., `0.01`.", thresholdStr)
		} else if threshold < 0 || threshold > 100 { responseText = fmt.Sprintf("⚠️ Поріг `%.4f%%` некоректний. Введіть від 0 до 100.", threshold)
		} else { SetUserFundingThreshold(chatID, threshold); responseText = fmt.Sprintf("✅ Поріг встановлено: `%.4f%%`.", threshold) }
		msg := tgbotapi.NewMessage(chatID, responseText); msg.ParseMode = tgbotapi.ModeMarkdown; sendAndLog(bot, msg, "set_funding_threshold_input", chatID)
		SetUserState(chatID, StateDefault); keyboard.ShowMainKeyboard(bot, chatID); return
	}

	if strings.HasPrefix(msgText, "/set_funding_threshold") {
		log.Printf("Обробка команди /set_funding_threshold для ChatID %d.", chatID); parts := strings.Fields(msgText)
		if len(parts) == 2 {
			thresholdStr := parts[1]; threshold, err := strconv.ParseFloat(thresholdStr, 64)
			var responseText string
			if err != nil { responseText = fmt.Sprintf("⚠️ Неправильний формат: `%s`. Приклад: `/set_funding_threshold 0.01`.", thresholdStr)
			} else if threshold < 0 || threshold > 100 { responseText = fmt.Sprintf("⚠️ Поріг `%.4f%%` некоректний. Введіть від 0 до 100.", threshold)
			} else { SetUserFundingThreshold(chatID, threshold); responseText = fmt.Sprintf("✅ Поріг встановлено: `%.4f%%`.", threshold) }
			msg := tgbotapi.NewMessage(chatID, responseText); msg.ParseMode = tgbotapi.ModeMarkdown; sendAndLog(bot, msg, "set_funding_threshold_cmd", chatID)
		} else { 
			currentThreshold := GetUserFundingThreshold(chatID)
			responseText := fmt.Sprintf("ℹ️ Поточний поріг: `%.4f%%`.\nВведіть нове значення (напр., `0.01`):", currentThreshold*100) 
			msg := tgbotapi.NewMessage(chatID, responseText); msg.ParseMode = tgbotapi.ModeMarkdown; sendAndLog(bot, msg, "set_funding_threshold_info_cmd", chatID)
			SetUserState(chatID, StateAwaitingFundingThreshold); log.Printf("Стан для ChatID %d -> %s", chatID, StateAwaitingFundingThreshold)
		}
		if len(parts) == 2 { keyboard.ShowMainKeyboard(bot, chatID) }; return
	}

	switch msgText {
	case keyboard.BtnWorkStart, "/start": commands.StartWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID)
	case keyboard.BtnWorkStop, "/stop": commands.StopWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case keyboard.BtnWorkDayOff, "/dayoff": commands.DayOff(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID)
	case keyboard.BtnMyGoal, "/goal":
		log.Printf("Обробка '%s' для %d", msgText, chatID); currentGoal, exists := GetUserGoal(chatID, srv, cfg) 
		if exists {
			goalInfoText := fmt.Sprintf("📌 Ваша поточна ціль:\n\nСума: `%.2f %s`\n(Ціль на %s %d)\nВстановлено: `%s`\n\nВведіть нову суму (напр. `15000 грн`).",
				currentGoal.Amount, currentGoal.Currency, monthNameUkrainian(currentGoal.SetDate.In(KyivLocation).Month()), 
				currentGoal.SetDate.In(KyivLocation).Year(), currentGoal.SetDate.In(KyivLocation).Format("02.01.2006"))
			msg := tgbotapi.NewMessage(chatID, goalInfoText); msg.ParseMode = tgbotapi.ModeMarkdown; sendAndLog(bot, msg, "view_goal_exists", chatID)
		} else {
			sendAndLog(bot, tgbotapi.NewMessage(chatID, "🎯 Введіть суму вашої цілі на поточний місяць (напр. `15000 грн`)."), "prompt_new_goal", chatID)
		}
		SetUserState(chatID, StateAwaitingGoalInput) 
	case keyboard.BtnCloseGoal, "/closegoal":
		// Викликаємо CloseUserGoal з closegoal.go
		CloseUserGoal(bot, chatID, srv, cfg) 
	case keyboard.BtnAddInvestment, "/add_investment":
		log.Printf("Обробка '%s' для %d", msgText, chatID)
		prompt := "➕ Введіть дані інвестиції:\n`ТИП, НАЗВА, СУМА ВАЛЮТА, ДАТА (РРРР-ММ-ДД)`\nПриклад:\n`Крипто, BTC, 0.5 BTC, 2023-10-25`"
		msg := tgbotapi.NewMessage(chatID, prompt); msg.ParseMode = tgbotapi.ModeMarkdown; sendAndLog(bot, msg, "add_investment_prompt", chatID)
		SetUserState(chatID, StateAwaitingInvestmentInput)
	case keyboard.BtnSetFundingThreshold:
		log.Printf("Обробка кнопки '%s' для ChatID %d.", keyboard.BtnSetFundingThreshold, chatID)
		currentThreshold := GetUserFundingThreshold(chatID); responseText := fmt.Sprintf("ℹ️ Поточний поріг: `%.4f%%`.\nВведіть новий (напр., `0.01`):", currentThreshold*100) 
		msg := tgbotapi.NewMessage(chatID, responseText); msg.ParseMode = tgbotapi.ModeMarkdown; sendAndLog(bot, msg, "set_funding_threshold_button_info", chatID)
		SetUserState(chatID, StateAwaitingFundingThreshold); log.Printf("Стан для ChatID %d -> %s", chatID, StateAwaitingFundingThreshold)
	case keyboard.BtnSpreads, "/spreads": 
		log.Printf("Обробка '%s' для ChatID %d.", msgText, chatID)
		initialReply := tgbotapi.NewMessage(chatID, "⏳ Пошук спредів..."); sentInitialMsg, errInitial := bot.Send(initialReply)
		var initialMsgID int
		if errInitial == nil && sentInitialMsg.MessageID != 0 { initialMsgID = sentInitialMsg.MessageID
		} else if errInitial != nil { log.Printf("Спреди: Не вдалося надіслати початкове повідомлення: %v", errInitial) }
		go HandleSpreadsCommand(bot, chatID, cfg, initialMsgID) 
		keyboard.ShowMainKeyboard(bot, chatID); return 
	case keyboard.BtnFundingRates, "/funding": 
		log.Printf("Обробка /funding для ChatID: %d.", chatID); currentFundingThreshold := GetUserFundingThreshold(chatID) 
		introText := "📊 **Funding Rates**\n" + fmt.Sprintf("_Поріг: `%.4f%%`._\n", currentFundingThreshold*100) + "_Ставки ... не враховують комісії._\n\nОберіть біржу:" 
		msg := tgbotapi.NewMessage(chatID, introText); msg.ParseMode = tgbotapi.ModeMarkdown
		msg.ReplyMarkup = keyboard.CreateFundingExchangeSelectionKeyboard(); sendAndLog(bot, msg, "funding_exchange_select_prompt", chatID)
	case "/motivation": 
		log.Printf("Обробка '/motivation' для ChatID %d", chatID)
		msg := tgbotapi.NewMessage(chatID, "Функція мотивації інтегрована після /stop.")
		sendAndLog(bot, msg, "motivation_info", chatID); keyboard.ShowMainKeyboard(bot, chatID)
	case keyboard.BtnProgress, "/report": ReportProgress(bot, update.Message, srv, cfg) 
	default:
		log.Printf("Не розпізнана команда або текст для ChatID %d: '%s'", chatID, msgText)
		if GetUserState(chatID) == StateDefault { 
			unknownCmdMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Невідома команда: '%s'.", msgText))
			sendAndLog(bot, unknownCmdMsg, "unknown_input_or_command", chatID); keyboard.ShowMainKeyboard(bot, chatID)
		}
	}
} 

// formatDurationToNextFunding має бути визначена тільки тут (або в utils)
func formatDurationToNextFunding(d time.Duration) string {
	isPast := false; if d < 0 { d = -d; isPast = true }
	hours := int(d.Hours()); minutes := int(d.Minutes()) % 60
	if hours == 0 && minutes == 0 {
		seconds := int(d.Seconds()) % 60
		if isPast { return "0с (минув)" }
		return fmt.Sprintf("%dс", seconds)
	}
	if isPast { return fmt.Sprintf("-%dг %dхв (минув)", hours, minutes) }
	return fmt.Sprintf("%dг %dхв", hours, minutes)
}
