package telegram

import (
	"fmt"
	"log"
	"sort"
	"strconv" // Потрібен для strconv.Atoi в checkTrustScore, якщо будемо парсити числові Trust Scores
	"strings"
	"sync"    // Повертаємо для горутин
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
)

// SpreadOpportunity ... (структура з ProfitPer100USD)
type SpreadOpportunity struct {
	CoinID          string
	BaseCurrency    string
	QuoteCurrency   string
	BuyExchange     string
	BuyPriceUSD     float64
	SellExchange    string
	SellPriceUSD    float64
	SpreadPercent   float64
	ProfitPer100USD float64 // ДОДАНО
	TrustScoreBuy   string
	TrustScoreSell  string
	TradeURLBuy     string
	TradeURLSell    string
	Comment         string
	Category        int
}

// isUserExchange ... (без змін)
func isUserExchange(exchangeIdentifier string, userExchanges []string) bool {
	normalizedIdentifier := strings.ToLower(strings.ReplaceAll(exchangeIdentifier, " ", "_"))
	for _, ue := range userExchanges {
		if normalizedIdentifier == ue {
			return true
		}
	}
	return false
}

// checkTrustScore ... (без змін)
func checkTrustScore(tickerTrustScore string, minTrustScoreConfig string) bool {
	if minTrustScoreConfig == "" || minTrustScoreConfig == "any" {
		return true
	}
	if tickerTrustScore == "" && minTrustScoreConfig != "" && minTrustScoreConfig != "any" {
		return false
	}
	switch strings.ToLower(minTrustScoreConfig) {
	case "green":
		return strings.ToLower(tickerTrustScore) == "green"
	case "yellow":
		return strings.ToLower(tickerTrustScore) == "green" || strings.ToLower(tickerTrustScore) == "yellow"
	case "red":
		return true
	default:
		log.Printf("Спреди: Невідомий або непідтримуваний формат SpreadMinTrustScore: '%s'. Фільтр TrustScore не застосовано для цього значення.", minTrustScoreConfig)
		return true
	}
}
// classifySpread ... (без змін)
func classifySpread(coinSymbol string, exchangeBuyID, exchangeSellID string, userExchangesMap map[string]bool, allCoinGeckoTickers []coingecko.CoinGeckoTickerDetail) (string, int) {
	buyIsUser := userExchangesMap[strings.ToLower(exchangeBuyID)]
	sellIsUser := userExchangesMap[strings.ToLower(exchangeSellID)]

	tokenOnUserOtherExchange := false
	var userExchangeWithToken string
	for _, ticker := range allCoinGeckoTickers {
		if strings.ToUpper(ticker.Base) == strings.ToUpper(coinSymbol) &&
			strings.ToUpper(ticker.Target) == "USDT" &&
			userExchangesMap[strings.ToLower(ticker.Market.Identifier)] &&
			strings.ToLower(ticker.Market.Identifier) != strings.ToLower(exchangeBuyID) &&
			strings.ToLower(ticker.Market.Identifier) != strings.ToLower(exchangeSellID) {
			tokenOnUserOtherExchange = true
			userExchangeWithToken = ticker.Market.Name
			break
		}
	}

	if buyIsUser && sellIsUser {
		return "✅ Обидві біржі у вашому списку!", 1
	}
	if buyIsUser {
		if tokenOnUserOtherExchange {
			return fmt.Sprintf("ℹ️ Купівля на вашій біржі. Продаж на '%s' (не ваша). Токен також є на вашій біржі '%s'.", exchangeSellID, userExchangeWithToken), 2
		}
		return fmt.Sprintf("⚠️ Купівля на вашій біржі. Продаж на '%s' (не ваша). Цього токена немає на інших ваших біржах.", exchangeSellID), 2
	}
	if sellIsUser {
		if tokenOnUserOtherExchange {
			return fmt.Sprintf("ℹ️ Продаж на вашій біржі. Купівля на '%s' (не ваша). Токен також є на вашій біржі '%s'.", exchangeBuyID, userExchangeWithToken), 2
		}
		return fmt.Sprintf("⚠️ Продаж на вашій біржі. Купівля на '%s' (не ваша). Цього токена немає на інших ваших біржах.", exchangeBuyID), 2
	}
	if tokenOnUserOtherExchange {
		return fmt.Sprintf("🔍 Спред між '%s' та '%s' (не ваші). Токен є на вашій біржі '%s'.", exchangeBuyID, exchangeSellID, userExchangeWithToken), 3
	}
	return fmt.Sprintf("🚫 Спред між '%s' та '%s' (не ваші). Токена немає на ваших біржах.", exchangeBuyID, exchangeSellID), 4
}

func HandleSpreadsCommand(bot *tgbotapi.BotAPI, chatID int64, cfg config.Config) {
	loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Пошук спредів... Це може зайняти деякий час (до кількох хвилин), будь ласка, зачекайте.")
	sentMsg, errSendLoad := bot.Send(loadingMsg)
	var originalMessageID int
	if errSendLoad == nil && sentMsg.MessageID != 0 {
		originalMessageID = sentMsg.MessageID
	} else {
		log.Printf("Спреди: Помилка надсилання повідомлення 'Пошук спредів': %v", errSendLoad)
	}

	log.Printf("Спреди: Початок пошуку. Топ монет: %d, Мін. спред: %.2f%%, Біржі користувача: %v, Мін. Trust Score: '%s'",
		cfg.SpreadCoinCount, cfg.SpreadMinPercentage, cfg.SpreadUserExchanges, cfg.SpreadMinTrustScore)

	topCoins, err := coingecko.GetTopMarketCapCoins(cfg.SpreadCoinCount, "usd")
	if err != nil {
		errorMsg := fmt.Sprintf("Помилка отримання списку топ-монет від CoinGecko: %v", err)
		if strings.Contains(err.Error(), "429") {
			errorMsg += "\n\n🚫 Схоже, ми досягли ліміту запитів до CoinGecko API. Спробуйте пізніше."
		}
		log.Printf("Спреди: %s", errorMsg)
		if originalMessageID != 0 {
			editMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, errorMsg)
			sendAndLog(bot, editMsg, "spreads_top_coins_error_edit", chatID)
		} else {
			sendAndLog(bot, tgbotapi.NewMessage(chatID, errorMsg), "spreads_top_coins_error_new", chatID)
		}
		keyboard.ShowMainKeyboard(bot, chatID)
		return
	}
	log.Printf("Спреди: Отримано %d монет для аналізу.", len(topCoins))

	var allFoundSpreads []SpreadOpportunity
	userExchangesMap := make(map[string]bool)
	for _, ex := range cfg.SpreadUserExchanges {
		userExchangesMap[ex] = true
	}

	processedCoins := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	delayBetweenCoinProcessing := 2500 * time.Millisecond
	
	for _, coinLoopVar := range topCoins {
		wg.Add(1)
		go func(currentCoin coingecko.CoinMarketData) { 
			defer wg.Done()
			
			mu.Lock()
			processedCoins++
			currentProcessedLocal := processedCoins
			mu.Unlock()

			if originalMessageID != 0 && currentProcessedLocal%5 == 0 {
				progressText := fmt.Sprintf("⏳ Пошук спредів... Оброблено %d/%d: %s...", currentProcessedLocal, len(topCoins), currentCoin.Name)
				editProgressMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, progressText)
				_, _ = bot.Send(editProgressMsg)
			}
			
			var coinAllTickersForThisCoin []coingecko.CoinGeckoTickerDetail
			for page := 1; page <= 1; page++ {
				tickersResponse, errTicker := coingecko.GetCoinTickers(currentCoin.ID, page)
				if errTicker != nil {
					if strings.Contains(errTicker.Error(), "429") {
						log.Printf("Спреди: Досягнуто ліміту CoinGecko при отриманні тікерів для %s.", currentCoin.ID)
					}
					break
				}
				if len(tickersResponse.Tickers) == 0 {
					break
				}
				coinAllTickersForThisCoin = append(coinAllTickersForThisCoin, tickersResponse.Tickers...)
			}

			var validTickers []coingecko.CoinGeckoTickerDetail
			for _, ticker := range coinAllTickersForThisCoin {
				if strings.ToUpper(ticker.Target) != "USDT" {
					continue
				}
				if !checkTrustScore(ticker.TrustScore, cfg.SpreadMinTrustScore) {
					continue
				}
				if priceUSD, ok := ticker.ConvertedLast["usd"]; ok && priceUSD > 0 {
					validTickers = append(validTickers, ticker)
				}
			}
			
			if len(validTickers) < 2 {
				return
			}

			for i := 0; i < len(validTickers); i++ {
				for j := i + 1; j < len(validTickers); j++ {
					tickerA := validTickers[i]
					tickerB := validTickers[j]

					if tickerA.Market.Identifier == tickerB.Market.Identifier {
						continue
					}

					priceA_USD := tickerA.ConvertedLast["usd"]
					priceB_USD := tickerB.ConvertedLast["usd"]
					
					var buyTicker, sellTicker coingecko.CoinGeckoTickerDetail
					var spreadPercent, profitPer100 float64

					if priceA_USD < priceB_USD {
						if priceA_USD == 0 { continue }
						buyTicker = tickerA
						sellTicker = tickerB
						spreadPercent = (priceB_USD/priceA_USD - 1) * 100
						profitPer100 = 100 * (priceB_USD/priceA_USD - 1)
					} else if priceB_USD < priceA_USD {
						if priceB_USD == 0 { continue }
						buyTicker = tickerB
						sellTicker = tickerA
						spreadPercent = (priceA_USD/priceB_USD - 1) * 100
						profitPer100 = 100 * (priceA_USD/priceB_USD - 1)
					} else {
						continue
					}

					if spreadPercent >= cfg.SpreadMinPercentage {
						comment, category := classifySpread(currentCoin.Symbol, buyTicker.Market.Identifier, sellTicker.Market.Identifier, userExchangesMap, coinAllTickersForThisCoin)
						
						if category == 4 && cfg.SpreadMinTrustScore != "" {
							continue
						}

						op := SpreadOpportunity{
							CoinID:          currentCoin.ID,
							BaseCurrency:    strings.ToUpper(buyTicker.Base),
							QuoteCurrency:   strings.ToUpper(buyTicker.Target),
							BuyExchange:     buyTicker.Market.Name,
							BuyPriceUSD:     buyTicker.ConvertedLast["usd"],
							SellExchange:    sellTicker.Market.Name,
							SellPriceUSD:    sellTicker.ConvertedLast["usd"],
							SpreadPercent:   spreadPercent,
							ProfitPer100USD: profitPer100, // Призначення розрахованого прибутку
							TrustScoreBuy:   buyTicker.TrustScore,
							TrustScoreSell:  sellTicker.TrustScore,
							TradeURLBuy:     buyTicker.TradeURL,
							TradeURLSell:    sellTicker.TradeURL,
							Comment:         comment,
							Category:        category,
						}
						mu.Lock()
						allFoundSpreads = append(allFoundSpreads, op)
						mu.Unlock()
					}
				}
			}
			time.Sleep(delayBetweenCoinProcessing)
		}(coinLoopVar)
	}
	wg.Wait()

	sort.SliceStable(allFoundSpreads, func(i, j int) bool {
		if allFoundSpreads[i].Category != allFoundSpreads[j].Category {
			return allFoundSpreads[i].Category < allFoundSpreads[j].Category
		}
		return allFoundSpreads[i].SpreadPercent > allFoundSpreads[j].SpreadPercent
	})
	
	var reportText strings.Builder
	reportText.WriteString(fmt.Sprintf("📈 **Знайдені Спреди (мін. %.2f%%, Топ-%d монет):**\n", cfg.SpreadMinPercentage, cfg.SpreadCoinCount))
	reportText.WriteString("_Увага: Ціни з CoinGecko, можуть відрізнятися від реальних. Завжди перевіряйте на біржах! Комісії не враховані._\n\n") // Додано про комісії

	if len(allFoundSpreads) == 0 {
		reportText.WriteString("Спредів, що відповідають вашим критеріям, не знайдено.")
	} else {
		limitSpreads := 7 
		displayedCount := 0
		for _, s := range allFoundSpreads {
			if displayedCount >= limitSpreads {
				break
			}
			tsBuyDisplay := s.TrustScoreBuy
			if tsBuyDisplay == "" { tsBuyDisplay = "N/A" }
			tsSellDisplay := s.TrustScoreSell
			if tsSellDisplay == "" { tsSellDisplay = "N/A" }

			reportText.WriteString(fmt.Sprintf(
				"**%s/%s (%.2f%%) | Прибуток на $100: `+$%.2f`**\n"+ // Додано ProfitPer100USD
					"  Купівля: *%s* (`$%.4f`, Trust: %s)\n"+
					"  Продаж: *%s* (`$%.4f`, Trust: %s)\n",
				s.BaseCurrency, s.QuoteCurrency, s.SpreadPercent, s.ProfitPer100USD,
				s.BuyExchange, s.BuyPriceUSD, tsBuyDisplay,
				s.SellExchange, s.SellPriceUSD, tsSellDisplay,
			))
			if s.Comment != "" {
				reportText.WriteString(fmt.Sprintf("  _%s_\n", s.Comment))
			}
			buyLink := ""
			if s.TradeURLBuy != "" { buyLink = fmt.Sprintf("[Купити](%s)", s.TradeURLBuy) }
			sellLink := ""
			if s.TradeURLSell != "" { sellLink = fmt.Sprintf("[Продати](%s)", s.TradeURLSell) }
			
			if buyLink != "" && sellLink != "" {
				reportText.WriteString(fmt.Sprintf("  %s | %s\n", buyLink, sellLink))
			} else if buyLink != "" {
				reportText.WriteString(fmt.Sprintf("  %s\n", buyLink))
			} else if sellLink != "" {
				reportText.WriteString(fmt.Sprintf("  %s\n", sellLink))
			}
			reportText.WriteString("\n")
			displayedCount++
		}
		if len(allFoundSpreads) > displayedCount {
			reportText.WriteString(fmt.Sprintf("... та ще %d спредів знайдено.", len(allFoundSpreads)-displayedCount))
		}
	}
	
	finalText := reportText.String()
	if len(finalText) > MaxTelegramMessageSize {
		log.Printf("Спреди: Повідомлення занадто довге (%d). Обрізаємо.", len(finalText))
		finalText = finalText[:MaxTelegramMessageSize-30] + "\n... (повідомлення обрізано)"
	}

	if originalMessageID != 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, originalMessageID)
		_, _ = bot.Send(deleteMsg) 
	}
	
	finalMsg := tgbotapi.NewMessage(chatID, finalText)
	finalMsg.ParseMode = tgbotapi.ModeMarkdown
	finalMsg.DisableWebPagePreview = true 
	sendAndLog(bot, finalMsg, "spreads_report_final", chatID)
	
	keyboard.ShowMainKeyboard(bot, chatID)
}

func min(a, b int) int {
	if a < b { return a }
	return b
}
