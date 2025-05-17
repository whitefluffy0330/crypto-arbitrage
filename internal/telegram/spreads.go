package telegram

import (
	"fmt"
	"log"
	"sort" // Повертаємо, бо використовується в розкоментованій логіці
	// "strconv" // ВИДАЛЕНО
	"strings"
	// "sync"    // Поки що не використовуємо горутини для обробки монет
	"time" // Повертаємо, використовується в delayBetweenCoinProcessing

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard" // Повертаємо, використовується в ShowMainKeyboard
)

// SpreadOpportunity - ВИЗНАЧЕННЯ ПОВЕРНУТО
type SpreadOpportunity struct {
	CoinID         string
	BaseCurrency   string
	QuoteCurrency  string
	BuyExchange    string
	BuyPriceUSD    float64
	SellExchange   string
	SellPriceUSD   float64
	SpreadPercent  float64
	ProfitPer100USD float64
	TrustScoreBuy  string
	TrustScoreSell string
	TradeURLBuy    string
	TradeURLSell   string
	Comment        string
	Category       int
}

// isUserExchange - ВИЗНАЧЕННЯ ПОВЕРНУТО
func isUserExchange(exchangeIdentifier string, userExchanges []string) bool {
	normalizedIdentifier := strings.ToLower(strings.ReplaceAll(exchangeIdentifier, " ", "_"))
	for _, ue := range userExchanges {
		if normalizedIdentifier == ue {
			return true
		}
	}
	return false
}

// checkTrustScore - ВИЗНАЧЕННЯ ПОВЕРНУТО
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
		log.Printf("Спреди: Невідомий формат SpreadMinTrustScore: '%s'. Фільтр не застосовано.", minTrustScoreConfig)
		return true
	}
}

// classifySpread - ВИЗНАЧЕННЯ ПОВЕРНУТО
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


func HandleSpreadsCommand(bot *tgbotapi.BotAPI, chatID int64, cfg config.Config, originalMessageID int) {
	// ---- ДІАГНОСТИКА ТИПУ (ЗАЛИШАЄМО ПОКИ ЩО) ----
	var testVar []coingecko.MarketCoin // Використовуємо правильний тип MarketCoin
	testVar, testErr := coingecko.GetTopMarketCapCoins(1, "usd")
	if testErr != nil {
		log.Printf("Спреди: ДІАГНОСТИКА: Помилка при виклику GetTopMarketCapCoins: %v", testErr)
	} else {
		if len(testVar) > 0 {
			log.Printf("Спреди: ДІАГНОСТИКА: GetTopMarketCapCoins викликано успішно. Перша монета ID: %s", testVar[0].ID)
		} else {
			log.Printf("Спреди: ДІАГНОСТИКА: GetTopMarketCapCoins викликано успішно, але список порожній.")
		}
	}
	// ---- КІНЕЦЬ ДІАГНОСТИКИ ----
	
	// Розкоментовуємо основну логіку, але поки БЕЗ ГОРУТИН для стабільності з CoinGecko API
	loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Пошук спредів... Це може зайняти деякий час, будь ласка, зачекайте.")
	// Надсилаємо повідомлення "Завантажую..." через originalMessageID (яке передається з handler.go)
	// або створюємо нове, якщо originalMessageID == 0
	var currentMessageID int
	if originalMessageID != 0 {
	    editLoadingMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, loadingMsg.Text)
	    if _, err := bot.Send(editLoadingMsg); err == nil {
	        currentMessageID = originalMessageID
	    } else {
	        log.Printf("Спреди: Помилка редагування на 'Завантажую...': %v. Надсилаю нове.", err)
            sentMsg, errSendLoad := bot.Send(loadingMsg)
            if errSendLoad == nil && sentMsg.MessageID != 0 {
                currentMessageID = sentMsg.MessageID
            } else {
                log.Printf("Спреди: Помилка надсилання повідомлення 'Пошук спредів': %v", errSendLoad)
            }
	    }
	} else {
        sentMsg, errSendLoad := bot.Send(loadingMsg)
        if errSendLoad == nil && sentMsg.MessageID != 0 {
            currentMessageID = sentMsg.MessageID
        } else {
            log.Printf("Спреди: Помилка надсилання повідомлення 'Пошук спредів': %v", errSendLoad)
        }
	}


	log.Printf("Спреди: Початок пошуку. Топ монет: %d, Мін. спред: %.2f%%, Біржі користувача: %v, Мін. Trust Score: '%s'",
		cfg.SpreadCoinCount, cfg.SpreadMinPercentage, cfg.SpreadUserExchanges, cfg.SpreadMinTrustScore)

	topCoins, err := coingecko.GetTopMarketCapCoins(cfg.SpreadCoinCount, "usd")
	if err != nil {
		errorMsgText := fmt.Sprintf("Помилка отримання списку топ-монет від CoinGecko: %v", err)
		if strings.Contains(err.Error(), "429") {
			errorMsgText += "\n\n🚫 Схоже, ми досягли ліміту запитів до CoinGecko API. Спробуйте пізніше."
		}
		log.Printf("Спреди: %s", errorMsgText)
		if currentMessageID != 0 {
			editMsg := tgbotapi.NewEditMessageText(chatID, currentMessageID, errorMsgText)
			sendAndLog(bot, editMsg, "spreads_top_coins_error_edit", chatID)
		} else {
			sendAndLog(bot, tgbotapi.NewMessage(chatID, errorMsgText), "spreads_top_coins_error_new", chatID)
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
	delayBetweenCoinProcessing := 6 * time.Second 
	hitRateLimit := false 

	for i, currentCoin := range topCoins { 
		if hitRateLimit { break }
		processedCoins++
		if currentMessageID != 0 && (processedCoins%1 == 0 || processedCoins == len(topCoins)) { 
			progressText := fmt.Sprintf("⏳ Пошук спредів... Оброблено %d/%d: %s...", processedCoins, len(topCoins), currentCoin.Name)
			editProgressMsg := tgbotapi.NewEditMessageText(chatID, currentMessageID, progressText)
			if _, errSend := bot.Send(editProgressMsg); errSend != nil {
				log.Printf("Спреди: Помилка оновлення прогресу: %v", errSend)
			}
		}
		
		var coinAllTickersForThisCoin []coingecko.CoinGeckoTickerDetail
		tickersResponse, errTicker := coingecko.GetCoinTickers(currentCoin.ID, 1)
		if errTicker != nil {
			if strings.Contains(errTicker.Error(), "429") {
				log.Printf("Спреди: Досягнуто ліміту CoinGecko при отриманні тікерів для %s. Завершуємо поточний пошук спредів.", currentCoin.ID)
				hitRateLimit = true 
			} else {
				log.Printf("Спреди: Помилка отримання тікерів для %s: %v. Пропускаємо монету.", currentCoin.ID, errTicker)
			}
			if i < len(topCoins)-1 && !hitRateLimit { time.Sleep(delayBetweenCoinProcessing) }
			continue 
		}
		if len(tickersResponse.Tickers) > 0 {
			coinAllTickersForThisCoin = append(coinAllTickersForThisCoin, tickersResponse.Tickers...)
		}
		
		var validTickers []coingecko.CoinGeckoTickerDetail
		for _, ticker := range coinAllTickersForThisCoin {
			if strings.ToUpper(ticker.Target) != "USDT" { continue }
			if !checkTrustScore(ticker.TrustScore, cfg.SpreadMinTrustScore) { continue }
			if priceUSD, ok := ticker.ConvertedLast["usd"]; ok && priceUSD > 0 {
				validTickers = append(validTickers, ticker)
			}
		}
		
		if len(validTickers) < 2 {
			if i < len(topCoins)-1 { time.Sleep(delayBetweenCoinProcessing) }
			continue
		}

		for k := 0; k < len(validTickers); k++ {
			for l := k + 1; l < len(validTickers); l++ {
				tickerA := validTickers[k]; tickerB := validTickers[l]
				if tickerA.Market.Identifier == tickerB.Market.Identifier { continue }
				priceA_USD := tickerA.ConvertedLast["usd"]; priceB_USD := tickerB.ConvertedLast["usd"]
				var buyTicker, sellTicker coingecko.CoinGeckoTickerDetail
				var spreadPercent, profitPer100 float64
				if priceA_USD < priceB_USD {
					if priceA_USD == 0 { continue }
					buyTicker = tickerA; sellTicker = tickerB
					spreadPercent = (priceB_USD/priceA_USD - 1) * 100
					profitPer100 = 100 * (priceB_USD/priceA_USD - 1)
				} else if priceB_USD < priceA_USD {
					if priceB_USD == 0 { continue }
					buyTicker = tickerB; sellTicker = tickerA
					spreadPercent = (priceA_USD/priceB_USD - 1) * 100
					profitPer100 = 100 * (priceA_USD/priceB_USD - 1)
				} else { continue }

				if spreadPercent >= cfg.SpreadMinPercentage {
					comment, category := classifySpread(currentCoin.Symbol, buyTicker.Market.Identifier, sellTicker.Market.Identifier, userExchangesMap, coinAllTickersForThisCoin)
					if category == 4 && cfg.SpreadMinTrustScore != "" { continue }
					op := SpreadOpportunity{
						CoinID: currentCoin.ID, BaseCurrency: strings.ToUpper(buyTicker.Base), QuoteCurrency: strings.ToUpper(buyTicker.Target),
						BuyExchange: buyTicker.Market.Name, BuyPriceUSD: buyTicker.ConvertedLast["usd"],
						SellExchange: sellTicker.Market.Name, SellPriceUSD: sellTicker.ConvertedLast["usd"],
						SpreadPercent: spreadPercent, ProfitPer100USD: profitPer100,
						TrustScoreBuy: buyTicker.TrustScore, TrustScoreSell: sellTicker.TrustScore,
						TradeURLBuy: buyTicker.TradeURL, TradeURLSell: sellTicker.TradeURL,
						Comment: comment, Category: category,
					}
					allFoundSpreads = append(allFoundSpreads, op)
				}
			}
		}
		if i < len(topCoins)-1 && !hitRateLimit { time.Sleep(delayBetweenCoinProcessing) }
	} 
	
	sort.SliceStable(allFoundSpreads, func(i, j int) bool {
		if allFoundSpreads[i].Category != allFoundSpreads[j].Category {
			return allFoundSpreads[i].Category < allFoundSpreads[j].Category
		}
		return allFoundSpreads[i].SpreadPercent > allFoundSpreads[j].SpreadPercent
	})
	
	var reportText strings.Builder
	reportText.WriteString(fmt.Sprintf("📈 **Знайдені Спреди (мін. %.2f%%, Топ-%d монет):**\n", cfg.SpreadMinPercentage, cfg.SpreadCoinCount))
	reportText.WriteString("_Увага: Ціни з CoinGecko, можуть відрізнятися від реальних. Завжди перевіряйте на біржах! Комісії не враховані._\n\n")

	if len(allFoundSpreads) == 0 {
		reportText.WriteString("Спредів, що відповідають вашим критеріям, не знайдено.")
		if hitRateLimit { 
			reportText.WriteString("\n\n⚠️ Досягнуто ліміту запитів до CoinGecko. Результати можуть бути неповними. Спробуйте пізніше.")
		}
	} else {
		// ... (код формування тексту звіту з спредами - без змін) ...
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
				"**%s/%s (%.2f%%) | Прибуток на $100: `+$%.2f`**\n"+
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
		if hitRateLimit { 
			reportText.WriteString("\n\n⚠️ Досягнуто ліміту запитів до CoinGecko. Показані результати можуть бути неповними.")
		}
	}
	
	finalText := reportText.String()
	if len(finalText) > MaxTelegramMessageSize {
		log.Printf("Спреди: Повідомлення занадто довге (%d). Обрізаємо.", len(finalText))
		finalText = finalText[:MaxTelegramMessageSize-30] + "\n... (повідомлення обрізано)"
	}

	if currentMessageID != 0 { // Використовуємо currentMessageID, отриманий після надсилання "Завантажую..."
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, currentMessageID)
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
