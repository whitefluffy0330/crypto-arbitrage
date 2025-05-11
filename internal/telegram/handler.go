package telegram

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time" // Потрібен для розрахунку тривалості до наступного фінансування

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

const (
	CallbackConfirmCloseGoal = "confirm_close_goal"
	CallbackCancelCloseGoal  = "cancel_close_goal"
)

// formatDurationToNextFunding форматує тривалість до наступного фінансування
func formatDurationToNextFunding(d time.Duration) string {
	if d < 0 {
		return "вже відбулася"
	}
	
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%d год %d хв", hours, minutes)
	}
	return fmt.Sprintf("%d хв", minutes)
}


func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	// ... (код для update.CallbackQuery залишається БЕЗ ЗМІН з відповіді #235) ...
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
			log.Printf("Передача Callback '%s' в goal.HandleCallback", callbackData); goal.HandleCallback(bot, update.CallbackQuery, srv, cfg)
		}
		answerCallback := tgbotapi.NewCallback(update.CallbackQuery.ID, ""); if _, err := bot.Request(answerCallback); err != nil { log.Printf("Помилка AnswerCallbackQuery: %v", err) }
		return
	}

	if update.Message == nil { return }
	chatID := update.Message.Chat.ID; msgText := update.Message.Text; userName := update.Message.From.UserName
	log.Printf("[%s] (%d): %s", userName, chatID, msgText)
	currentState := GetUserState(chatID)

	if currentState == StateAwaitingGoalInput { /* ... (код без змін з #235) ... */ } else if currentState == StateAwaitingInvestmentInput { /* ... (код без змін з #235) ... */ }
	
	switch msgText {
	case "/start", "🔁 Старт": commands.StartWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/stop", "⛔️ Стоп": commands.StopWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/dayoff", "🏖 Вихідний": commands.DayOff(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/goal", "🎯 Моя ціль": /* ... (код без змін з #235) ... */
	case "/closegoal", "❌ Закрити ціль": /* ... (код без змін з #235) ... */
	case "/add_investment": /* ... (код без змін з #235) ... */
	
	case "/funding", "💹 Funding Rates":
		log.Printf("Обробка команди /funding для ChatID: %d", chatID)
		loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Завантажую топ-монети з CoinGecko та ставки з Binance...")
		sentMsg, _ := bot.Send(loadingMsg)
		var fundingReportText string

		topCoins, errCoinGecko := coingecko.GetTopMarketCapCoins(20, "usd") // Топ-20
		if errCoinGecko != nil {
			log.Printf("Помилка CoinGecko: %v", errCoinGecko); fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати топ-монети: %v", errCoinGecko)
		} else if len(topCoins) == 0 { fundingReportText = "Не знайдено топ-монет на CoinGecko."
		} else {
			var targetBinanceSymbols []string
			for _, coin := range topCoins { targetBinanceSymbols = append(targetBinanceSymbols, strings.ToUpper(coin.Symbol) + "USDT") }
			log.Printf("Сформовано %d цільових символів для Binance: %v", len(targetBinanceSymbols), targetBinanceSymbols)

			allRatesMap, errBinance := binance.GetFundingRates()
			if errBinance != nil { log.Printf("Помилка Binance: %v", errBinance); fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати ставки Binance: %v", errBinance)
			} else if len(allRatesMap) == 0 { fundingReportText = "Ставки Binance недоступні."
			} else {
				var relevantRatesSlice []binance.FundingInfo
				for _, s := range targetBinanceSymbols { if r, ok := allRatesMap[s]; ok { relevantRatesSlice = append(relevantRatesSlice, r) } }
				log.Printf("Знайдено %d релевантних ставок Binance.", len(relevantRatesSlice))

				if len(relevantRatesSlice) == 0 { fundingReportText = "Не знайдено ставок фінансування для топ-монет на Binance."
				} else {
					sort.SliceStable(relevantRatesSlice, func(i, j int) bool { /* ... (логіка сортування без змін з #233) ... */ 
						if relevantRatesSlice[i].LastFundingRate > 0 && relevantRatesSlice[j].LastFundingRate <= 0 { return true }; if relevantRatesSlice[i].LastFundingRate <= 0 && relevantRatesSlice[j].LastFundingRate > 0 { return false }; if relevantRatesSlice[i].LastFundingRate > 0 && relevantRatesSlice[j].LastFundingRate > 0 { return relevantRatesSlice[i].LastFundingRate > relevantRatesSlice[j].LastFundingRate }; return relevantRatesSlice[i].LastFundingRate < relevantRatesSlice[j].LastFundingRate
					})
					
					var sb strings.Builder
					sb.WriteString("📊 **Ставки Фінансування (Binance Futures) для Топ-Монет:**\n\n")
					
					limit := 7; now := time.Now()

					sb.WriteString("📈 **Найвищі Позитивні (вигідно Short):**\n")
					foundPositive := false; positiveCount := 0;
					for _, info := range relevantRatesSlice {
						if positiveCount >= limit { break }
						if info.LastFundingRate > 0.001 { // Невеликий поріг для відображення
							profitPer100 := 100 * (info.LastFundingRate / 100.0) // info.LastFundingRate - це вже % * 100, тому ділимо на 100
							                                                   // Якщо LastFundingRate = 0.01 (це 0.01%), то $100 * 0.0001 = $0.01
							nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation)
							durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv))
							sb.WriteString(fmt.Sprintf("`%s`: Ставка: `%.4f%%`\n  Mark: `%.2f`\n  Наст. виплата: `%s` (через %s)\n  Оцінка для $100 (Short): `+$%.2f`\n\n",
								info.Symbol, info.LastFundingRate, info.MarkPrice,
								nextTimeKyiv.Format("15:04 (02.01)"), durationToNext,
								profitPer100))
							positiveCount++; foundPositive = true
						}
					}
					if !foundPositive { sb.WriteString("_Немає значних позитивних ставок._\n") }; sb.WriteString("\n")

					sb.WriteString("📉 **Найбільш Негативні (вигідно Long):**\n")
					foundNegative := false; negativeCount := 0;
					for i := len(relevantRatesSlice) - 1; i >= 0; i-- {
						if negativeCount >= limit { break }
						info := relevantRatesSlice[i]
						if info.LastFundingRate < -0.001 { // Невеликий поріг для відображення
							payoutPer100 := 100 * (-info.LastFundingRate / 100.0) // Беремо позитивне значення
							nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation)
							durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv))
							sb.WriteString(fmt.Sprintf("`%s`: Ставка: `%.4f%%`\n  Mark: `%.2f`\n  Наст. виплата: `%s` (через %s)\n  Оцінка для $100 (Long): `+$%.2f`\n\n",
								info.Symbol, info.LastFundingRate, info.MarkPrice,
								nextTimeKyiv.Format("15:04 (02.01)"), durationToNext,
								payoutPer100))
							negativeCount++; foundNegative = true
						}
					}
					if !foundNegative { sb.WriteString("_Немає значних негативних ставок._\n") }; fundingReportText = sb.String()
				}
			}
		}
		if sentMsg.MessageID != 0 { editText := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, fundingReportText); editText.ParseMode = tgbotapi.ModeMarkdown; bot.Send(editText) } else { finalMsg := tgbotapi.NewMessage(chatID, fundingReportText); finalMsg.ParseMode = tgbotapi.ModeMarkdown; bot.Send(finalMsg) }
		keyboard.ShowMainKeyboard(bot, chatID)

	case "/motivation": motivationText := motivation.GetRandomMotivation(); msg := tgbotapi.NewMessage(chatID, motivationText); if _, err := bot.Send(msg); err != nil { log.Printf("Помилка мотивації: %v", err) }; keyboard.ShowMainKeyboard(bot, chatID) 
	case "/report", "📊 Прогрес": ReportProgress(bot, update.Message, srv, cfg) 
	default: log.Printf("Не розпізнана команда: [%s]: %s.", userName, msgText); keyboard.ShowMainKeyboard(bot, chatID)
	}
}

// Тіла інших функцій (SetUserState, GetUserState, HandleMyGoalCommand, CloseUserGoal, AddInvestment) мають бути повними з попередніх відповідей
