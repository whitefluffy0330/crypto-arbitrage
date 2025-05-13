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
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/binance"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation" // Імпорт можна видалити, якщо /motivation теж видаляється
	gsheets "google.golang.org/api/sheets/v4"
)

const (
	CallbackConfirmCloseGoal = "confirm_close_goal"
	CallbackCancelCloseGoal  = "cancel_close_goal"
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

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		messageID := update.CallbackQuery.Message.MessageID
		userName := update.CallbackQuery.From.UserName
		callbackData := update.CallbackQuery.Data
		log.Printf("Callback від [%s](%d): Data=%s, MsgID=%d", userName, chatID, callbackData, messageID)
		var callbackResponseText string
		originalMessageText := ""
		if update.CallbackQuery.Message != nil {
			originalMessageText = update.CallbackQuery.Message.Text
		}
		switch callbackData {
		case CallbackConfirmCloseGoal:
			log.Printf("Підтверджено закриття цілі для %d", chatID)
			err := DeleteUserGoal(chatID, srv, cfg)
			if err != nil {
				if strings.Contains(err.Error(), "не знайдено активної цілі") {
					callbackResponseText = "ℹ️ Активну ціль не знайдено."
				} else {
					callbackResponseText = "⚠️ Помилка закриття цілі."
					log.Printf("DeleteUserGoal err: %v", err)
				}
			} else {
				callbackResponseText = "✅ Ціль успішно закрито!"
			}
			editText := tgbotapi.NewEditMessageText(chatID, messageID, originalMessageText+"\n\n"+callbackResponseText)
			editText.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, editText, "confirm_close_goal_edit", chatID)
			keyboard.ShowMainKeyboard(bot, chatID)
		case CallbackCancelCloseGoal:
			log.Printf("Скасовано закриття цілі для %d", chatID)
			callbackResponseText = "🚫 Закриття цілі скасовано."
			editText := tgbotapi.NewEditMessageText(chatID, messageID, originalMessageText+"\n\n"+callbackResponseText)
			editText.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, editText, "cancel_close_goal_edit", chatID)
			keyboard.ShowMainKeyboard(bot, chatID)
		default:
			log.Printf("Передача Callback '%s' в goal.HandleCallback", callbackData)
			goal.HandleCallback(bot, update.CallbackQuery, srv, cfg)
		}
		answerCallbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, callbackResponseText)
		requestAndLog(bot, answerCallbackCfg, "answer_callback", chatID)
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
	log.Printf("Поточний стан для ChatID %d: '%s'", chatID, currentState)

	switch currentState {
	case StateAwaitingGoalInput:
		HandleGoalInput(bot, update.Message, srv, cfg)
		SetUserState(chatID, StateDefault)
		keyboard.ShowMainKeyboard(bot, chatID)
		return
	case StateAwaitingInvestmentInput:
		HandleInvestmentInput(bot, update.Message, srv, cfg)
		SetUserState(chatID, StateDefault)
		keyboard.ShowMainKeyboard(bot, chatID)
		return
	case StateAwaitingFundingThreshold:
		thresholdStr := strings.TrimSpace(update.Message.Text)
		threshold, err := strconv.ParseFloat(thresholdStr, 64)
		if err != nil {
			responseText := fmt.Sprintf("⚠️ Неправильний формат числа для порогу: `%s`. Введіть число, наприклад, `0.01` для 0.01%%.", thresholdStr)
			msg := tgbotapi.NewMessage(chatID, responseText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, msg, "set_funding_threshold_error", chatID)
		} else if threshold < 0 || threshold > 100 {
			responseText := fmt.Sprintf("⚠️ Поріг `%.4f%%` не є коректним. Введіть значення від 0 до 100.", threshold)
			msg := tgbotapi.NewMessage(chatID, responseText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, msg, "set_funding_threshold_range_error", chatID)
		} else {
			SetUserFundingThreshold(chatID, threshold)
			responseText := fmt.Sprintf("✅ Поріг для ставок фінансування встановлено: `%.4f%%`.\nКоманда `/funding` тепер буде показувати ставки вище (або нижче для негативних) цього значення.", threshold)
			msg := tgbotapi.NewMessage(chatID, responseText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, msg, "set_funding_threshold_success", chatID)
		}
		SetUserState(chatID, StateDefault)
		keyboard.ShowMainKeyboard(bot, chatID)
		return
	}

	if strings.HasPrefix(msgText, "/set_funding_threshold") {
		log.Printf("Обробка команди /set_funding_threshold для ChatID %d.", chatID)
		parts := strings.Fields(msgText)
		if len(parts) == 2 {
			thresholdStr := parts[1]
			threshold, err := strconv.ParseFloat(thresholdStr, 64)
			if err != nil {
				responseText := fmt.Sprintf("⚠️ Неправильний формат числа для порогу: `%s`. Використовуйте команду так: `/set_funding_threshold 0.01`.", thresholdStr)
				msg := tgbotapi.NewMessage(chatID, responseText)
				msg.ParseMode = tgbotapi.ModeMarkdown
				sendAndLog(bot, msg, "set_funding_threshold_cmd_error", chatID)
			} else if threshold < 0 || threshold > 100 {
				responseText := fmt.Sprintf("⚠️ Поріг `%.4f%%` не є коректним. Введіть значення від 0 до 100.", threshold)
				msg := tgbotapi.NewMessage(chatID, responseText)
				msg.ParseMode = tgbotapi.ModeMarkdown
				sendAndLog(bot, msg, "set_funding_threshold_cmd_range_error", chatID)
			} else {
				SetUserFundingThreshold(chatID, threshold)
				responseText := fmt.Sprintf("✅ Поріг для ставок фінансування встановлено: `%.4f%%`.", threshold)
				msg := tgbotapi.NewMessage(chatID, responseText)
				msg.ParseMode = tgbotapi.ModeMarkdown
				sendAndLog(bot, msg, "set_funding_threshold_cmd_success", chatID)
				keyboard.ShowMainKeyboard(bot, chatID)
			}
		} else {
			currentThreshold := GetUserFundingThreshold(chatID)
			responseText := fmt.Sprintf("ℹ️ Поточний поріг для ставок фінансування: `%.4f%%`.\nЩоб встановити новий, введіть нове значення (наприклад, `0.01`):", currentThreshold)
			msg := tgbotapi.NewMessage(chatID, responseText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, msg, "set_funding_threshold_info", chatID)
			SetUserState(chatID, StateAwaitingFundingThreshold)
			log.Printf("Стан для ChatID %d -> %s", chatID, StateAwaitingFundingThreshold)
		}
		return
	}

	switch msgText {
	case keyboard.BtnWorkStart, "/start":
		commands.StartWork(bot, update.Message, srv, cfg)
		keyboard.ShowMainKeyboard(bot, chatID)
	case keyboard.BtnWorkStop, "/stop":
		commands.StopWork(bot, update.Message, srv, cfg)
		keyboard.ShowMainKeyboard(bot, chatID)
	case keyboard.BtnWorkDayOff, "/dayoff":
		commands.DayOff(bot, update.Message, srv, cfg)
		keyboard.ShowMainKeyboard(bot, chatID)
	case keyboard.BtnMyGoal, "/goal":
		log.Printf("Обробка '%s' для %d", msgText, chatID)
		currentGoal, exists := GetUserGoal(chatID, srv, cfg)
		if exists {
			goalInfoText := fmt.Sprintf(
				"📌 Ваша поточна ціль:\n\nСума: `%.2f %s`\n(Ціль на %s %d)\nВстановлено: `%s`\n\nЯкщо бажаєте встановити нову ціль, поточна буде автоматично заархівована.\nЩоб встановити нову, просто введіть суму (напр. `15000 грн`).",
				currentGoal.Amount, currentGoal.Currency,
				monthNameUkrainian(currentGoal.SetDate.In(sheets.KyivLocation).Month()),
				currentGoal.SetDate.In(sheets.KyivLocation).Year(),
				currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006"),
			)
			msg := tgbotapi.NewMessage(chatID, goalInfoText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, msg, "view_goal_exists", chatID)
			goal.HandleMyGoalCommand(bot, chatID)
			SetUserState(chatID, StateAwaitingGoalInput)
		} else {
			goal.HandleMyGoalCommand(bot, chatID)
			SetUserState(chatID, StateAwaitingGoalInput)
		}
	case keyboard.BtnCloseGoal, "/closegoal":
		log.Printf("Обробка '%s' для %d", msgText, chatID)
		currentGoal, exists := GetUserGoal(chatID, srv, cfg)
		if exists {
			confirmationText := fmt.Sprintf(
				"❓ Ви дійсно хочете закрити поточну ціль?\n\nСума: `%.2f %s`\n(Ціль на %s %d)\nВстановлено: `%s`",
				currentGoal.Amount, currentGoal.Currency,
				monthNameUkrainian(currentGoal.SetDate.In(sheets.KyivLocation).Month()),
				currentGoal.SetDate.In(sheets.KyivLocation).Year(),
				currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006"),
			)
			msg := tgbotapi.NewMessage(chatID, confirmationText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			msg.ReplyMarkup = keyboard.CreateConfirmationKeyboard(CallbackConfirmCloseGoal, CallbackCancelCloseGoal)
			sendAndLog(bot, msg, "close_goal_confirm_prompt", chatID)
		} else {
			msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної фінансової цілі для закриття.")
			sendAndLog(bot, msg, "close_goal_no_active", chatID)
			keyboard.ShowMainKeyboard(bot, chatID)
		}
	case keyboard.BtnAddInvestment, "/add_investment":
		log.Printf("Обробка '%s' для %d", msgText, chatID)
		prompt := "➕ Введіть дані про вашу нову інвестицію у форматі:\n`ТИП, НАЗВА/СИМВОЛ, СУМА ВАЛЮТА, ДАТА (РРРР-ММ-ДД)`\n\nНаприклад:\n`Крипто-холд, BTC, 0.5 BTC, 2023-10-25`\n`Акція, META, 1000 USD, 2024-01-15`"
		msg := tgbotapi.NewMessage(chatID, prompt)
		msg.ParseMode = tgbotapi.ModeMarkdown
		sendAndLog(bot, msg, "add_investment_prompt", chatID)
		SetUserState(chatID, StateAwaitingInvestmentInput)
	case keyboard.BtnSetFundingThreshold: // Обробка кнопки "⚙️ Поріг Funding"
		log.Printf("Обробка кнопки '%s' для ChatID %d.", keyboard.BtnSetFundingThreshold, chatID)
		currentThreshold := GetUserFundingThreshold(chatID)
		responseText := fmt.Sprintf("ℹ️ Поточний поріг для ставок фінансування: `%.4f%%`.\nЩоб встановити новий, введіть нове значення (наприклад, `0.01`):", currentThreshold)
		msg := tgbotapi.NewMessage(chatID, responseText)
		msg.ParseMode = tgbotapi.ModeMarkdown
		sendAndLog(bot, msg, "set_funding_threshold_button_info", chatID)
		SetUserState(chatID, StateAwaitingFundingThreshold)
		log.Printf("Стан для ChatID %d -> %s (через кнопку)", chatID, StateAwaitingFundingThreshold)
	
	case keyboard.BtnSpreads, "/spreads": // Новий case для кнопки "Спреди" та команди /spreads
		log.Printf("Обробка '%s' для ChatID %d.", msgText, chatID)
		responseText := "📈 Функція моніторингу спредів наразі в розробці. Слідкуйте за оновленнями!"
		msg := tgbotapi.NewMessage(chatID, responseText)
		sendAndLog(bot, msg, "spreads_wip", chatID)
		keyboard.ShowMainKeyboard(bot, chatID) // Показуємо головну клавіатуру після інформаційного повідомлення

	case keyboard.BtnFundingRates, "/funding":
		log.Printf("Обробка '%s' для ChatID: %d", msgText, chatID)
		loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Завантажую топ-монети з CoinGecko та ставки з Binance Futures...")
		sentMsgObj, errSendLoad := bot.Send(loadingMsg)
		if errSendLoad != nil {
			log.Printf("ПОМИЛКА send loadingMsg /funding: %v", errSendLoad)
		}
		currentFundingThreshold := GetUserFundingThreshold(chatID)
		var fundingReportText string
		topCoins, errCoinGecko := coingecko.GetTopMarketCapCoins(20, "usd")
		if errCoinGecko != nil {
			fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати топ-монети з CoinGecko: %v", errCoinGecko)
		} else if len(topCoins) == 0 {
			fundingReportText = "Не знайдено топ-монет на CoinGecko."
		} else {
			var targetBinanceSymbols []string
			for _, coin := range topCoins { targetBinanceSymbols = append(targetBinanceSymbols, strings.ToUpper(coin.Symbol)+"USDT") }
			allRatesMap, errBinance := binance.GetFundingRates()
			if errBinance != nil {
				fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати ставки фінансування з Binance: %v", errBinance)
			} else if len(allRatesMap) == 0 {
				fundingReportText = "Дані про ставки фінансування з Binance недоступні або порожні."
			} else {
				var relevantRatesSlice []binance.FundingInfo
				for _, symbol := range targetBinanceSymbols { if rateInfo, ok := allRatesMap[symbol]; ok { relevantRatesSlice = append(relevantRatesSlice, rateInfo) } }
				if len(relevantRatesSlice) == 0 {
					fundingReportText = "Не знайдено ставок фінансування для топ-монет на Binance Futures."
				} else {
					sort.SliceStable(relevantRatesSlice, func(i, j int) bool {
						rateI := relevantRatesSlice[i].LastFundingRate; rateJ := relevantRatesSlice[j].LastFundingRate
						if rateI > 0 && rateJ > 0 { return rateI > rateJ }
						if rateI < 0 && rateJ < 0 { return rateI < rateJ }
						return rateI > rateJ
					})
					var sb strings.Builder
					sb.WriteString("📊 **Funding Rates (Binance Futures) для Топ-Монет (CoinGecko):**\n")
					sb.WriteString(fmt.Sprintf("_Поточний поріг відображення: `%.4f%%`._\n", currentFundingThreshold))
					sb.WriteString("_Ставки фінансування – це періодичні платежі... Розрахунок... не враховує торгові комісії._\n\n")
					limit := 7; posCount := 0; negCount := 0
					sb.WriteString("📈 **Найвищі Позитивні Ставки (Long платить Short):**\n_Для цих пар власники Short-позицій отримують ставку..._\n------------------------------\n"); foundPos := false
					for _, info := range relevantRatesSlice {
						if posCount >= limit { break }
						if info.LastFundingRate > currentFundingThreshold {
							profitPer100 := 100 * (info.LastFundingRate / 100.0); nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation); durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv))
							sb.WriteString(fmt.Sprintf("`%s` (Mark: `$%.2f`)\n  Ставка: `+%.4f%%`\n  Дохід на $100 Short: `+$%.2f`\n  Наступна: `%s` (через %s)\n", info.Symbol, info.MarkPrice, info.LastFundingRate, profitPer100, nextTimeKyiv.Format("15:04 (02.01)"), durationToNext)); sb.WriteString("------------------------------\n"); posCount++; foundPos = true
						}
					}
					if !foundPos { sb.WriteString(fmt.Sprintf("_Немає позитивних ставок вище `%.4f%%`._\n", currentFundingThreshold)) }
					sb.WriteString("\n")
					sb.WriteString("📉 **Найбільш Негативні Ставки (Short платить Long):**\n_Для цих пар власники Long-позицій отримують ставку..._\n------------------------------\n"); foundNeg := false
					for i := len(relevantRatesSlice) - 1; i >= 0; i-- {
						if negCount >= limit { break }
						info := relevantRatesSlice[i]
						if info.LastFundingRate < -currentFundingThreshold {
							payoutPer100 := 100 * (-info.LastFundingRate / 100.0); nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation); durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv))
							sb.WriteString(fmt.Sprintf("`%s` (Mark: `$%.2f`)\n  Ставка: `%.4f%%`\n  Дохід на $100 Long: `+$%.2f`\n  Наступна: `%s` (через %s)\n", info.Symbol, info.MarkPrice, info.LastFundingRate, payoutPer100, nextTimeKyiv.Format("15:04 (02.01)"), durationToNext)); sb.WriteString("------------------------------\n"); negCount++; foundNeg = true
						}
					}
					if !foundNeg { sb.WriteString(fmt.Sprintf("_Немає негативних ставок нижче `-%.4f%%`._\n", currentFundingThreshold)) }
					fundingReportText = sb.String()
				}
			}
		}
		if sentMsgObj.MessageID != 0 && errSendLoad == nil {
			editText := tgbotapi.NewEditMessageText(chatID, sentMsgObj.MessageID, fundingReportText); editText.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, editText, "funding_report_edit", chatID)
		} else {
			finalMsg := tgbotapi.NewMessage(chatID, fundingReportText); finalMsg.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, finalMsg, "funding_report_new", chatID)
		}
		keyboard.ShowMainKeyboard(bot, chatID)
	
	// Команда /motivation ВИДАЛЕНА звідси.
	// Якщо ви все ж хочете залишити текстову команду /motivation, розкоментуйте цей блок:
	/*
	case "/motivation":
		log.Printf("Обробка '/motivation' для ChatID %d", chatID)
		// Потрібно переконатися, що пакет motivation імпортовано, якщо він використовується
		// import "github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
		motivationText := motivation.GetRandomMotivation()
		msg := tgbotapi.NewMessage(chatID, motivationText)
		sendAndLog(bot, msg, "motivation_cmd", chatID)
		keyboard.ShowMainKeyboard(bot, chatID)
	*/

	case keyboard.BtnProgress, "/report":
		log.Printf("Обробка '%s' для ChatID %d", msgText, chatID)
		ReportProgress(bot, update.Message, srv, cfg)
	default:
		log.Printf("Не розпізнана команда або текст для ChatID %d: '%s'", chatID, msgText)
		unknownCmdMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Вибачте, команда або текст '%s' не оброблені. Скористайтеся кнопками меню.", msgText))
		sendAndLog(bot, unknownCmdMsg, "unknown_input_or_command", chatID)
		keyboard.ShowMainKeyboard(bot, chatID)
	}
}

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

// monthNameUkrainian тут не потрібна, вона є в report.go
