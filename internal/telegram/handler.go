package telegram

import (
	"fmt"
	"log"
	"sort" 
	"strings"
	// "time" // Більше не потрібен тут напряму

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/binance"
	// Додаємо імпорт coingecko
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

const ( CallbackConfirmCloseGoal = "confirm_close_goal"; CallbackCancelCloseGoal  = "cancel_close_goal" )

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	if update.CallbackQuery != nil { /* ... код обробки CallbackQuery без змін ... */ return }
	if update.Message == nil { return }
	chatID := update.Message.Chat.ID; msgText := update.Message.Text; userName := update.Message.From.UserName
	log.Printf("[%s] (%d): %s", userName, chatID, msgText)
	currentState := GetUserState(chatID)

	if currentState == StateAwaitingGoalInput { /* ... код керування станом для цілі без змін ... */ } else if currentState == StateAwaitingInvestmentInput { /* ... код керування станом для інвестиції без змін ... */ }
	
	switch msgText {
	case "/start", "🔁 Старт": commands.StartWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/stop", "⛔️ Стоп": commands.StopWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/dayoff", "🏖 Вихідний": commands.DayOff(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/goal", "🎯 Моя ціль": /* ... код без змін ... */
	case "/closegoal", "❌ Закрити ціль": /* ... код без змін ... */
	case "/add_investment": /* ... код без змін ... */
	
	case "/funding", "💹 Funding Rates":
		log.Printf("Обробка команди /funding для ChatID: %d", chatID)
		loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Завантажую топ-монети з CoinGecko та ставки з Binance...")
		sentMsg, _ := bot.Send(loadingMsg)
		var fundingReportText string

		// 1. Отримуємо топ-монети з CoinGecko
		topCoins, errCoinGecko := coingecko.GetTopMarketCapCoins(20, "usd") // Беремо топ-20
		if errCoinGecko != nil {
			log.Printf("Помилка отримання топ-монет з CoinGecko: %v", errCoinGecko)
			fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати список топ-монет з CoinGecko: %v", errCoinGecko)
		} else if len(topCoins) == 0 {
			fundingReportText = "Не знайдено топ-монет на CoinGecko."
		} else {
			// 2. Формуємо список символів для Binance (наприклад, BTC -> BTCUSDT)
			var targetBinanceSymbols []string
			// Також зберігаємо мапу CoinGecko ID -> Binance Symbol для відображення
			cgIDToBinanceSymbol := make(map[string]string) 
			for _, coin := range topCoins {
				// Просте перетворення, може потребувати уточнення для деяких символів
				binanceSymbol := strings.ToUpper(coin.Symbol) + "USDT" 
				targetBinanceSymbols = append(targetBinanceSymbols, binanceSymbol)
				cgIDToBinanceSymbol[coin.ID] = binanceSymbol // Може бути корисно для майбутнього
			}
			log.Printf("Сформовано %d цільових символів для Binance: %v", len(targetBinanceSymbols), targetBinanceSymbols)

			// 3. Отримуємо всі ставки фінансування з Binance
			allRatesMap, errBinance := binance.GetFundingRates()
			if errBinance != nil {
				log.Printf("Помилка отримання funding rates з Binance: %v", errBinance)
				fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати ставки фінансування з Binance: %v", errBinance)
			} else if len(allRatesMap) == 0 {
				fundingReportText = "Інформація про ставки фінансування з Binance наразі недоступна."
			} else {
				// 4. Фільтруємо ставки Binance за нашим списком топ-монет
				var relevantRatesSlice []binance.FundingInfo
				for _, binanceSymbol := range targetBinanceSymbols {
					if rateInfo, ok := allRatesMap[binanceSymbol]; ok {
						relevantRatesSlice = append(relevantRatesSlice, rateInfo)
					}
				}
				log.Printf("Знайдено %d релевантних ставок на Binance.", len(relevantRatesSlice))

				if len(relevantRatesSlice) == 0 {
					fundingReportText = "Не знайдено ставок фінансування для топ-монет на Binance."
				} else {
					// 5. Сортуємо релевантні ставки
					sort.SliceStable(relevantRatesSlice, func(i, j int) bool {
						if relevantRatesSlice[i].LastFundingRate > 0 && relevantRatesSlice[j].LastFundingRate <= 0 { return true }
						if relevantRatesSlice[i].LastFundingRate <= 0 && relevantRatesSlice[j].LastFundingRate > 0 { return false }
						if relevantRatesSlice[i].LastFundingRate > 0 && relevantRatesSlice[j].LastFundingRate > 0 { return relevantRatesSlice[i].LastFundingRate > relevantRatesSlice[j].LastFundingRate }
						return relevantRatesSlice[i].LastFundingRate < relevantRatesSlice[j].LastFundingRate
					})
					
					// 6. Формуємо звіт
					var sb strings.Builder
					sb.WriteString("📊 **Ставки Фінансування для Топ-Монет (Binance Futures):**\n_(оцінка для позиції $100 за 8 год.)_\n\n")
					
					limit := 7 
					positiveCount := 0
					negativeCount := 0

					sb.WriteString("📈 **Найвищі Позитивні Ставки (вигідно Short):**\n")
					foundPositive := false
					for _, info := range relevantRatesSlice {
						if positiveCount >= limit { break }
						if info.LastFundingRate > 0.001 { // Невеликий поріг
							profitPer100 := info.LastFundingRate 
							nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation)
							sb.WriteString(fmt.Sprintf("`%s`: `%.4f%%` (+$%.2f) (Next: %s)\n", info.Symbol, info.LastFundingRate, profitPer100, nextTimeKyiv.Format("15:04")))
							positiveCount++
							foundPositive = true
						}
					}
					if !foundPositive { sb.WriteString("_Не знайдено значних позитивних ставок._\n") }
					sb.WriteString("\n")

					sb.WriteString("📉 **Найбільш Негативні Ставки (вигідно Long):**\n")
					foundNegative := false
					// Шукаємо з кінця відсортованого масиву для найбільш негативних
					for i := len(relevantRatesSlice) - 1; i >= 0; i-- {
						if negativeCount >= limit { break }
						info := relevantRatesSlice[i]
						if info.LastFundingRate < -0.001 { // Невеликий поріг
							payoutPer100 := -info.LastFundingRate 
							nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation)
							sb.WriteString(fmt.Sprintf("`%s`: `%.4f%%` (+$%.2f) (Next: %s)\n", info.Symbol, info.LastFundingRate, payoutPer100, nextTimeKyiv.Format("15:04")))
							negativeCount++
							foundNegative = true
						}
					}
					if !foundNegative { sb.WriteString("_Не знайдено значних негативних ставок._\n") }
					fundingReportText = sb.String()
				}
			}
		}
		// Оновлюємо повідомлення або надсилаємо нове
		if sentMsg.MessageID != 0 { editText := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, fundingReportText); editText.ParseMode = tgbotapi.ModeMarkdown; bot.Send(editText) } else { finalMsg := tgbotapi.NewMessage(chatID, fundingReportText); finalMsg.ParseMode = tgbotapi.ModeMarkdown; bot.Send(finalMsg) }
		keyboard.ShowMainKeyboard(bot, chatID)

	case "/motivation": /* ... код без змін ... */
	case "/report", "📊 Прогрес": /* ... код без змін ... */
	default: /* ... код без змін ... */
	}
}
