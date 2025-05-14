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
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	gsheets "google.golang.org/api/sheets/v4"
)

const (
	CallbackConfirmCloseGoal = "confirm_close_goal"
	CallbackCancelCloseGoal  = "cancel_close_goal"
	MaxTelegramMessageSize   = 4096 // Максимальна довжина повідомлення в Telegram
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
				// keyboard.ShowMainKeyboard(bot, chatID) // Не показуємо тут, щоб не перебивати можливий діалог
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
	case keyboard.CallbackFundingBinance:
		rates, err = binance.GetFundingRates()
	case keyboard.CallbackFundingBybit:
		rates, err = bybit.GetFundingRates()
	case keyboard.CallbackFundingOKX:
		rates, err = okx.GetFundingRates()
	case keyboard.CallbackFundingMEXC:
		rates, err = mexc.GetFundingRates()
	case keyboard.CallbackFundingBitget: 
		rates, err = bitget.GetFundingRates()
	default:
		log.Printf("Невідомий callback для фандингу: %s", exchangeCallbackPrefix)
		errorText := fmt.Sprintf("Помилка: невідома біржа для запиту (%s).", exchangeName)
		if originalMessageID != 0 {
			finalEditMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, errorText)
			sendAndLog(bot, finalEditMsg, "funding_unknown_exchange_edit", chatID)
		} else {
			sendAndLog(bot, tgbotapi.NewMessage(chatID, errorText), "funding_unknown_exchange_new", chatID)
		}
		keyboard.ShowMainKeyboard(bot, chatID)
		return
	}

	currentFundingThreshold := GetUserFundingThreshold(chatID)
	var reportText string

	if err != nil {
		log.Printf("Помилка отримання даних з %s для /funding: %v", exchangeName, err)
		reportText = fmt.Sprintf("⚠️ %s: не вдалося завантажити дані.\nПомилка: %v", exchangeName, err)
	} else if len(rates) == 0 {
		reportText = fmt.Sprintf("ℹ️ %s: дані про ставки фінансування порожні або не знайдено відповідних пар (поріг: `%.4f%%`).", exchangeName, currentFundingThreshold)
	} else {
		sort.SliceStable(rates, func(i, j int) bool {
			rateI := rates[i].LastFundingRate
			rateJ := rates[j].LastFundingRate
			if rateI > 0 && rateJ > 0 { return rateI > rateJ }
			if rateI < 0 && rateJ < 0 { return rateI < rateJ }
			return rateI > rateJ
		})

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📊 **Funding Rates (%s):**\n", exchangeName))
		sb.WriteString(fmt.Sprintf("_Поточний поріг відображення: `%.4f%%`._\n", currentFundingThreshold))
		sb.WriteString("_Ставки фінансування – це періодичні платежі між трейдерами. Прогнозований дохід/витрати розраховуються на один період фінансування (зазвичай 8 годин) і не враховують торгові комісії._\n\n")

		limit := 5 
		posCount := 0
		negCount := 0

		sb.WriteString("📈 **Найвищі Позитивні Ставки (Long платить Short):**\n")
		sb.WriteString("_Для цих пар власники Short-позицій отримують ставку..._\n")
		sb.WriteString("------------------------------\n"); foundPos := false
		for _, info := range rates {
			if posCount >= limit { break }
			if info.LastFundingRate > currentFundingThreshold {
				profitPer100 := 100 * (info.LastFundingRate / 100.0)
				
				var nextFundingDisplay string
				if info.NextFundingTime.IsZero() || info.NextFundingTime.Unix() <= 0 {
					nextFundingDisplay = "N/A"
				} else {
					nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation)
					durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv))
					nextFundingDisplay = fmt.Sprintf("%s (через %s)", nextTimeKyiv.Format("15:04 (02.01)"), durationToNext)
				}

				sb.WriteString(fmt.Sprintf(
					"`%s` (%s, Mark: `$%.2f`)\n  Ставка: `+%.4f%%`\n  Прогноз доходу на $100 Short до наст. виплати: `+$%.2f`\n  Наступна: %s\n",
					info.Symbol, info.Exchange, info.MarkPrice, info.LastFundingRate, profitPer100,
					nextFundingDisplay,
				)); sb.WriteString("------------------------------\n"); posCount++; foundPos = true
			}
		}
		if !foundPos { sb.WriteString(fmt.Sprintf("_Немає позитивних ставок вище `%.4f%%`._\n", currentFundingThreshold)) }
		sb.WriteString("\n")

		sb.WriteString("📉 **Найбільш Негативні Ставки (Short платить Long):**\n")
		sb.WriteString("_Для цих пар власники Long-позицій отримують ставку..._\n")
		sb.WriteString("------------------------------\n"); foundNeg := false
		
		tempNegRates := []exchanges.UnifiedFundingRateInfo{}
		for _, info := range rates {
			if info.LastFundingRate < -currentFundingThreshold {
				tempNegRates = append(tempNegRates, info)
			}
		}
		sort.SliceStable(tempNegRates, func(i, j int) bool {
			return tempNegRates[i].LastFundingRate < tempNegRates[j].LastFundingRate
		})

		for _, info := range tempNegRates {
			if negCount >= limit { break }
				payoutPer100 := 100 * (-info.LastFundingRate / 100.0)
				
				var nextFundingDisplay string
				if info.NextFundingTime.IsZero() || info.NextFundingTime.Unix() <= 0 {
					nextFundingDisplay = "N/A"
				} else {
					nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation)
					durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv))
					nextFundingDisplay = fmt.Sprintf("%s (через %s)", nextTimeKyiv.Format("15:04 (02.01)"), durationToNext)
				}

				sb.WriteString(fmt.Sprintf(
					"`%s` (%s, Mark: `$%.2f`)\n  Ставка: `%.4f%%`\n  Прогноз доходу на $100 Long до наст. виплати: `+$%.2f`\n  Наступна: %s\n",
					info.Symbol, info.Exchange, info.MarkPrice, info.LastFundingRate, payoutPer100,
					nextFundingDisplay,
				)); sb.WriteString("------------------------------\n"); negCount++; foundNeg = true
		}
		if !foundNeg { sb.WriteString(fmt.Sprintf("_Немає негативних ставок нижче `-%.4f%%`._\n", currentFundingThreshold)) }
		reportText = sb.String()
	}
	
	var finalMessageText = reportText
	if len(finalMessageText) > MaxTelegramMessageSize {
		log.Printf("Повідомлення для фандингу %s занадто довге (%d). Обрізаємо.", exchangeName, len(finalMessageText))
		finalMessageText = finalMessageText[:MaxTelegramMessageSize-30] + "\n... (повідомлення обрізано)"
	}

	if originalMessageID != 0 {
		finalEditMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, finalMessageText)
		finalEditMsg.ParseMode = tgbotapi.ModeMarkdown
		finalEditMsg.ReplyMarkup = nil 
		if _, err := bot.Send(finalEditMsg); err != nil {
			log.Printf("Помилка фінального редагування повідомлення для %s (ID: %d): %v. Спроба надіслати нове.", exchangeName, originalMessageID, err)
			newFinalMsg := tgbotapi.NewMessage(chatID, finalMessageText)
			newFinalMsg.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, newFinalMsg, "funding_report_new_after_edit_fail", chatID)
		}
	} else { // Якщо originalMessageID == 0 (не вдалося надіслати/відредагувати "завантажую")
		log.Printf("originalMessageID is 0 для фандингу %s, надсилаю звіт новим повідомленням.", exchangeName)
		newFinalMsg := tgbotapi.NewMessage(chatID, finalMessageText)
		newFinalMsg.ParseMode = tgbotapi.ModeMarkdown
		sendAndLog(bot, newFinalMsg, "funding_report_new_no_orig_msgid", chatID)
	}
	keyboard.ShowMainKeyboard(bot, chatID)
}


func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		userName := update.CallbackQuery.From.UserName
		callbackData := update.CallbackQuery.Data
		
		// Логуємо MessageID з CallbackQuery.Message, якщо воно існує
		var msgIDForLog int
		if update.CallbackQuery.Message != nil {
			msgIDForLog = update.CallbackQuery.Message.MessageID
		} else {
			log.Printf("ПОПЕРЕДЖЕННЯ: update.CallbackQuery.Message is nil для ChatID %d, Data: %s", chatID, callbackData)
		}
		log.Printf("Callback від [%s](%d): Data=%s, OrigMsgID=%d", userName, chatID, callbackData, msgIDForLog)
		
		switch callbackData {
		case keyboard.CallbackFundingBinance:
			handleFundingExchangeSelection(bot, update.CallbackQuery, chatID, "Binance", keyboard.CallbackFundingBinance)
			return 
		case keyboard.CallbackFundingBybit:
			handleFundingExchangeSelection(bot, update.CallbackQuery, chatID, "Bybit", keyboard.CallbackFundingBybit)
			return
		case keyboard.CallbackFundingOKX:
			handleFundingExchangeSelection(bot, update.CallbackQuery, chatID, "OKX", keyboard.CallbackFundingOKX)
			return
		case keyboard.CallbackFundingMEXC:
			handleFundingExchangeSelection(bot, update.CallbackQuery, chatID, "MEXC", keyboard.CallbackFundingMEXC)
			return
		case keyboard.CallbackFundingBitget: 
			handleFundingExchangeSelection(bot, update.CallbackQuery, chatID, "Bitget", keyboard.CallbackFundingBitget)
			return
		}

		var callbackResponseText string
		originalMessageText := ""
		var originalMsgID int // Для редагування
		if update.CallbackQuery.Message != nil {
			originalMessageText = update.CallbackQuery.Message.Text
			originalMsgID = update.CallbackQuery.Message.MessageID
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
			if originalMsgID != 0 {
				editText := tgbotapi.NewEditMessageText(chatID, originalMsgID, originalMessageText+"\n\n"+callbackResponseText)
				editText.ParseMode = tgbotapi.ModeMarkdown
				sendAndLog(bot, editText, "confirm_close_goal_edit", chatID)
			} else {
				sendAndLog(bot, tgbotapi.NewMessage(chatID, callbackResponseText), "confirm_close_goal_new", chatID)
			}
			keyboard.ShowMainKeyboard(bot, chatID) 
		case CallbackCancelCloseGoal:
			log.Printf("Скасовано закриття цілі для %d", chatID)
			callbackResponseText = "🚫 Закриття цілі скасовано."
			if originalMsgID != 0 {
				editText := tgbotapi.NewEditMessageText(chatID, originalMsgID, originalMessageText+"\n\n"+callbackResponseText)
				editText.ParseMode = tgbotapi.ModeMarkdown
				sendAndLog(bot, editText, "cancel_close_goal_edit", chatID)
			} else {
				sendAndLog(bot, tgbotapi.NewMessage(chatID, callbackResponseText), "cancel_close_goal_new", chatID)
			}
			keyboard.ShowMainKeyboard(bot, chatID)
		default:
			log.Printf("Передача Callback '%s' в goal.HandleCallback або невідомий callback", callbackData)
			// Якщо це callback для цілей, goal.HandleCallback має сам обробити відповідь
			// і, можливо, відредагувати повідомлення.
			goal.HandleCallback(bot, update.CallbackQuery, srv, cfg) 
			// Не надсилаємо answerCallbackCfg тут, якщо goal.HandleCallback сам це робить.
			return // Повертаємося, щоб не надсилати зайвий answerCallbackCfg
