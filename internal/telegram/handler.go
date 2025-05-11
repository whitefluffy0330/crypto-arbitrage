package telegram

import (
	"fmt"
	"log"
	"sort"
	"strings"
	// "time" // Не потрібен тут напряму

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	// ВИПРАВЛЕНО ШЛЯХ ІМПОРТУ для binance
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/binance" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
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

// Допоміжні функції для логування надсилання повідомлень
func sendAndLog(bot *tgbotapi.BotAPI, c tgbotapi.Chattable, commandName string, chatID int64) { /* ... код без змін ... */ }
func requestAndLog(bot *tgbotapi.BotAPI, c tgbotapi.CallbackConfig, commandName string, chatID int64) { /* ... код без змін ... */ }


func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	// Обробка CallbackQuery (код без змін з відповіді #241)
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID; messageID := update.CallbackQuery.Message.MessageID; userName := update.CallbackQuery.From.UserName; callbackData := update.CallbackQuery.Data
		log.Printf("Callback від [%s](%d): Data=%s, MsgID=%d", userName, chatID, callbackData, messageID); var callbackResponseText string
		originalMessageText := ""; if update.CallbackQuery.Message != nil { originalMessageText = update.CallbackQuery.Message.Text }
		switch callbackData {
		case CallbackConfirmCloseGoal:
			log.Printf("Підтверджено закриття цілі для %d", chatID); err := DeleteUserGoal(chatID, srv, cfg)
			if err != nil { if strings.Contains(err.Error(), "не знайдено активної цілі") { callbackResponseText = "ℹ️ Активну ціль не знайдено." } else { callbackResponseText = "⚠️ Помилка закриття цілі."; log.Printf("DeleteUserGoal err: %v", err) } } else { callbackResponseText = "✅ Ціль успішно закрито!" }
			editText := tgbotapi.NewEditMessageText(chatID, messageID, originalMessageText+"\n\n"+callbackResponseText); editText.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, editText, "confirm_close_goal_edit", chatID)
			keyboard.ShowMainKeyboard(bot, chatID)
		case CallbackCancelCloseGoal:
			log.Printf("Скасовано закриття цілі для %d", chatID); callbackResponseText = "🚫 Закриття цілі скасовано."
			editText := tgbotapi.NewEditMessageText(chatID, messageID, originalMessageText+"\n\n"+callbackResponseText); editText.ParseMode = tgbotapi.ModeMarkdown
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

	// Обробка звичайних повідомлень (код без змін з відповіді #241)
	if update.Message == nil { return }
	chatID := update.Message.Chat.ID; msgText := update.Message.Text; userName := update.Message.From.UserName
	log.Printf("[%s] (%d): %s", userName, chatID, msgText)
	currentState := GetUserState(chatID)

	if currentState == StateAwaitingGoalInput { /* ... */ } else if currentState == StateAwaitingInvestmentInput { /* ... */ }
	
	// Обробка Команд / Кнопок (код без змін з відповіді #241, але з використанням ВИПРАВЛЕНИХ імпортів)
	switch msgText {
	case "/start", "🔁 Старт": commands.StartWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/stop", "⛔️ Стоп": commands.StopWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/dayoff", "🏖 Вихідний": commands.DayOff(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/goal", "🎯 Моя ціль": 
		log.Printf("Обробка /goal для %d", chatID); currentGoal, exists := GetUserGoal(chatID, srv, cfg) 
		if exists { log.Printf("Знайдено ціль для %d: %+v", chatID, currentGoal); goalInfoText := fmt.Sprintf("📌 Ваша поточна ціль:\n\nСума: `%.2f %s`\n(Ціль на %s %d)\nВстановлено: `%s`\n\n...", currentGoal.Amount, currentGoal.Currency, monthNameUkrainian(currentGoal.SetDate.In(sheets.KyivLocation).Month()), currentGoal.SetDate.In(sheets.KyivLocation).Year(), currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006")); msg := tgbotapi.NewMessage(chatID, goalInfoText); msg.ParseMode = tgbotapi.ModeMarkdown; sendAndLog(bot, msg, "view_goal_exists", chatID); keyboard.ShowMainKeyboard(bot, chatID) 
		} else { log.Printf("Активна ціль для %d не знайдена.", chatID); goal.HandleMyGoalCommand(bot, chatID); SetUserState(chatID, StateAwaitingGoalInput); log.Printf("Стан %d -> awaiting_goal", chatID) }
	case "/closegoal", "❌ Закрити ціль": 
		log.Printf("Обробка /closegoal для %d", chatID); currentGoal, exists := GetUserGoal(chatID, srv, cfg) 
		if exists { confirmationText := fmt.Sprintf("❓ Впевнені?\nСума: `%.2f %s`\nВстановлено: `%s`", currentGoal.Amount, currentGoal.Currency, currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006")); msg := tgbotapi.NewMessage(chatID, confirmationText); msg.ParseMode = tgbotapi.ModeMarkdown; msg.ReplyMarkup = keyboard.CreateConfirmationKeyboard(CallbackConfirmCloseGoal, CallbackCancelCloseGoal); sendAndLog(bot, msg, "close_goal_confirm_prompt", chatID)
		} else { msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної цілі."); sendAndLog(bot, msg, "close_goal_no_active", chatID); keyboard.ShowMainKeyboard(bot, chatID) }
	case "/add_investment": 
		log.Printf("Обробка /add_investment для %d", chatID); prompt := "➕ Введіть інвестицію:\n`ТИП, НАЗВА, СУМА ВАЛЮТА, ДАТА (РРРР-ММ-ДД)`\nПриклад: `Акція, AAPL, 1000 USD, 2024-03-15`"; msg := tgbotapi.NewMessage(chatID, prompt); msg.ParseMode = tgbotapi.ModeMarkdown; sendAndLog(bot, msg, "add_investment_prompt", chatID); SetUserState(chatID, StateAwaitingInvestmentInput); log.Printf("Стан %d -> awaiting_investment", chatID)
	
	case "/funding", "💹 Funding Rates": 
		log.Printf("Обробка команди /funding для ChatID: %d", chatID); 
		loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Завантажую топ-монети з CoinGecko та ставки з Binance..."); 
		sentMsgObj, errSendLoad := bot.Send(loadingMsg); if errSendLoad != nil {log.Printf("ПОМИЛКА send loadingMsg /funding: %v", errSendLoad)}
		var fundingReportText string
		topCoins, errCoinGecko := coingecko.GetTopMarketCapCoins(20, "usd")
		if errCoinGecko != nil { log.Printf("Помилка CoinGecko: %v", errCoinGecko); fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати топ-монети: %v", errCoinGecko)
		} else if len(topCoins) == 0 { fundingReportText = "Не знайдено топ-монет на CoinGecko."
		} else {
			var targetBinanceSymbols []string; for _, coin := range topCoins { targetBinanceSymbols = append(targetBinanceSymbols, strings.ToUpper(coin.Symbol) + "USDT") }
			log.Printf("Сформовано %d цільових символів: %v", len(targetBinanceSymbols), targetBinanceSymbols)
			allRatesMap, errBinance := binance.GetFundingRates() // ВИКОРИСТАННЯ binance
			if errBinance != nil { log.Printf("Помилка Binance: %v", errBinance); fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати ставки Binance: %v", errBinance)
			} else if len(allRatesMap) == 0 { fundingReportText = "Ставки Binance недоступні."
			} else {
				var relevantRatesSlice []binance.FundingInfo; for _, s := range targetBinanceSymbols { if r, ok := allRatesMap[s]; ok { relevantRatesSlice = append(relevantRatesSlice, r) } }
				log.Printf("Знайдено %d релевантних ставок Binance.", len(relevantRatesSlice))
				if len(relevantRatesSlice) == 0 { fundingReportText = "Не знайдено ставок для топ-монет на Binance."
				} else {
					sort.SliceStable(relevantRatesSlice, func(i, j int) bool { if relevantRatesSlice[i].LastFundingRate > 0 && relevantRatesSlice[j].LastFundingRate <= 0 { return true }; if relevantRatesSlice[i].LastFundingRate <= 0 && relevantRatesSlice[j].LastFundingRate > 0 { return false }; if relevantRatesSlice[i].LastFundingRate > 0 && relevantRatesSlice[j].LastFundingRate > 0 { return relevantRatesSlice[i].LastFundingRate > relevantRatesSlice[j].LastFundingRate }; return relevantRatesSlice[i].LastFundingRate < relevantRatesSlice[j].LastFundingRate }) // ВИКОРИСТАННЯ sort
					var sb strings.Builder; sb.WriteString("📊 **Funding Rates (Binance Futures) для Топ-Монет:**\n\n"); limit := 7; posCount := 0; negCount := 0
					sb.WriteString("📈 **Найвищі Позитивні (вигідно Short):**\n"); foundPos := false
					for _, info := range relevantRatesSlice { if posCount >= limit { break }; if info.LastFundingRate > 0.001 { profit := 100*(info.LastFundingRate/100.0); nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation); durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv)); sb.WriteString(fmt.Sprintf("`%s`: `%.4f%%` (+$%.2f) (Next: %s, %s)\n", info.Symbol, info.LastFundingRate, profit, nextTimeKyiv.Format("15:04 (02.01)"), durationToNext)); posCount++; foundPos = true } }
					if !foundPos { sb.WriteString("_Немає значних позитивних ставок._\n") }; sb.WriteString("\n")
					sb.WriteString("📉 **Найбільш Негативні (вигідно Long):**\n"); foundNeg := false
					for i := len(relevantRatesSlice) - 1; i >= 0; i-- { if negCount >= limit { break }; info := relevantRatesSlice[i]; if info.LastFundingRate < -0.001 { payout := 100*(-info.LastFundingRate/100.0); nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation); durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv)); sb.WriteString(fmt.Sprintf("`%s`: `%.4f%%` (+$%.2f) (Next: %s, %s)\n", info.Symbol, info.LastFundingRate, payout, nextTimeKyiv.Format("15:04 (02.01)"), durationToNext)); negCount++; foundNeg = true } }
					if !foundNeg { sb.WriteString("_Немає значних негативних ставок._\n") }; fundingReportText = sb.String()
				}
			}
		}
		if sentMsgObj.MessageID != 0 && errSendLoad == nil { editText := tgbotapi.NewEditMessageText(chatID, sentMsgObj.MessageID, fundingReportText); editText.ParseMode = tgbotapi.ModeMarkdown; sendAndLog(bot, editText, "funding_report_edit", chatID) } else { finalMsg := tgbotapi.NewMessage(chatID, fundingReportText); finalMsg.ParseMode = tgbotapi.ModeMarkdown; sendAndLog(bot, finalMsg, "funding_report_new", chatID) }
		keyboard.ShowMainKeyboard(bot, chatID)

	case "/motivation": 
		motivationText := motivation.GetRandomMotivation(); // ВИКОРИСТАННЯ motivation
		msg := tgbotapi.NewMessage(chatID, motivationText); 
		sendAndLog(bot, msg, "motivation", chatID)
		keyboard.ShowMainKeyboard(bot, chatID) 
	case "/report", "📊 Прогрес":
		ReportProgress(bot, update.Message, srv, cfg) 
	default:
		log.Printf("Не розпізнана команда: [%s]: %s.", userName, msgText)
		// keyboard.ShowMainKeyboard(bot, chatID) // Не показуємо клавіатуру, якщо команда не розпізнана, щоб не спамити
	}
}
