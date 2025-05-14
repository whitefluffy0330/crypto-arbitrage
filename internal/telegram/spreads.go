package telegram

import (
	"fmt"
	"log"
	"sort"
	// "strconv" // ВИДАЛЕНО, оскільки strconv.Atoi використовувався лише в checkTrustScore, який поки що не парсить числа
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko" // Імпорт має бути тут
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
)

// SpreadOpportunity зберігає інформацію про знайдений спред
type SpreadOpportunity struct {
	CoinID         string  // ID монети на CoinGecko (наприклад, "bitcoin")
	BaseCurrency   string  // Символ базової валюти (наприклад, "BTC")
	QuoteCurrency  string  // Символ котирувальної валюти (наприклад, "USDT")
	BuyExchange    string  // Назва біржі для купівлі
	BuyPriceUSD    float64 // Ціна купівлі в USD
	SellExchange   string  // Назва біржі для продажу
	SellPriceUSD   float64 // Ціна продажу в USD
	SpreadPercent  float64 // Розрахований відсоток спреду
	TrustScoreBuy  string  // Рейтинг надійності біржі купівлі ("green", "yellow", "red", "" якщо немає)
	TrustScoreSell string  // Рейтинг надійності біржі продажу
	TradeURLBuy    string  // Пряме посилання на торгову пару для купівлі
	TradeURLSell   string  // Пряме посилання на торгову пару для продажу
	Comment        string  // Коментар щодо "ваших бірж" та наявності токена
	Category       int     // Категорія пріоритету (1-найвищий, 2, 3, 4)
}

func isUserExchange(exchangeIdentifier string, userExchanges []string) bool {
	normalizedIdentifier := strings.ToLower(strings.ReplaceAll(exchangeIdentifier, " ", "_"))
	for _, ue := range userExchanges {
		if normalizedIdentifier == ue {
			return true
		}
	}
	return false
}

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
		// strconv.Atoi тут не потрібен, бо ми порівнюємо рядки "green", "yellow"
		return true
	}
}

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
			strings.ToLower(ticker.Market.Identifier) != strings.ToLower(exchangeSellID) { // ВИПРАВЛЕНО: sellExchangeID -> exchangeSellID
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
			return fmt.Sprintf("ℹ️ Купівля на вашій біржі. Продаж на '%s' (не ваша). Токен також є на вашій біржі '%s'.", exchangeSellID, userExchangeWithToken), 2 // ВИПРАВЛЕНО
		}
		return fmt.Sprintf("⚠️ Купівля на вашій біржі. Продаж на '%s' (не ваша). Цього токена немає на інших ваших біржах.", exchangeSellID), 2 // ВИПРАВЛЕНО
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

	// Тип topCoins визначається функцією GetTopMarketCapCoins як []coingecko.CoinMarketData
	topCoins, err := coingecko.GetTopMarketCapCoins(cfg.SpreadCoinCount, "usd")
	if err != nil {
		log.Printf("Спреди: Помилка отримання топ монет: %v", err)
		errorText := fmt.Sprintf("Помилка отримання списку топ-монет: %v", err)
		if originalMessageID != 0 {
			editMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, errorText)
			sendAndLog(bot, editMsg, "spreads_top_coins_error_edit", chatID)
		} else {
			sendAndLog(bot, tgbotapi.NewMessage(chatID, errorText), "spreads_top_coins_error_new", chatID)
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

	for _, coinLoopVar := range topCoins { // Змінено ім'я змінної циклу, щоб уникнути конфлікту з параметром горутини
		wg.Add(1)
		// Передаємо копію coinLoopVar в горутину
		go func(c coingecko.CoinMarketData) { // Параметр c тепер має чітко визначений тип
			defer wg.Done()

			mu.Lock()
			processedCoins++
			currentProcessedLocal := processedCoins
			mu.Unlock()

			if originalMessageID != 0 && currentProcessedLocal%5 == 0 {
				progressText := fmt.Sprintf("⏳ Пошук спредів... Оброблено %d з %d монет (%s)...", currentProcessedLocal, len(topCoins), c.Name)
				editProgressMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, progressText)
				_, _ = bot.Send(editProgressMsg)
			}

			var coinAllTickersForThisCoin []coingecko.CoinGeckoTickerDetail
			for page := 1; page <= 3; page++ {
				tickersResponse, errTicker := coingecko.GetCoinTickers(c.ID, page)
				if errTicker != nil {
					break
				}
				if len(tickersResponse.Tickers) == 0 {
					break
				}
				coinAllTickersForThisCoin = append(coinAllTickersForThisCoin, tickersResponse.Tickers...)
				if len(tickersResponse.Tickers) < 100 {
					break
				}
				time.Sleep(1200 * time.Millisecond)
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
					var spreadPercent float64

					if priceA_USD < priceB_USD {
						buyTicker = tickerA
						sellTicker = tickerB
						spreadPercent = (priceB_USD/priceA_USD - 1) * 100
					} else if priceB_USD < priceA_USD {
						buyTicker = tickerB
						sellTicker = tickerA
						spreadPercent = (priceA_USD/priceB_USD - 1) * 100
					} else {
						continue
					}

					if spreadPercent >= cfg.SpreadMinPercentage {
						comment, category := classifySpread(c.Symbol, buyTicker.Market.Identifier, sellTicker.Market.Identifier, userExchangesMap, coinAllTickersForThisCoin)
						
						if category == 4 { // Не додаємо спреди найнижчої категорії
							continue
						}

						op := SpreadOpportunity{
							CoinID:         c.ID,
							BaseCurrency:   strings.ToUpper(buyTicker.Base),
							QuoteCurrency:  strings.ToUpper(buyTicker.Target),
							BuyExchange:    buyTicker.Market.Name,
							BuyPriceUSD:    buyTicker.ConvertedLast["usd"],
							SellExchange:   sellTicker.Market.Name,
							SellPriceUSD:   sellTicker.ConvertedLast["usd"],
							SpreadPercent:  spreadPercent,
							TrustScoreBuy:  buyTicker.TrustScore,
							TrustScoreSell: sellTicker.TrustScore,
							TradeURLBuy:    buyTicker.TradeURL,
							TradeURLSell:   sellTicker.TradeURL,
							Comment:        comment,
							Category:       category,
						}
						mu.Lock()
						allFoundSpreads = append(allFoundSpreads, op)
						mu.Unlock()
					}
				}
			}
			time.Sleep(1500 * time.Millisecond)
		}(coinLoopVar) // Передаємо копію coinLoopVar
	}
	wg.Wait()

	sort.SliceStable(allFoundSpreads, func(i, j int) bool {
		if allFoundSpreads[i].Category != allFoundSpreads[j].Category {
			return allFoundSpreads[i].Category < allFoundSpreads[j].Category
		}
		return allFoundSpreads[i].SpreadPercent > allFoundSpreads[j].SpreadPercent
	})

	var reportText strings.Builder
	reportText.WriteString(fmt.Sprintf("📈 **Знайдені Спреди (мін. %.2f%%, Топ-%d монет):**\n\n", cfg.SpreadMinPercentage, cfg.SpreadCoinCount))

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
				"**%s/%s (%.2f%%)**\n"+
					"  Купівля: *%s* (`$%.4f`, Trust: %s)\n"+
					"  Продаж: *%s* (`$%.4f`, Trust: %s)\n",
				s.BaseCurrency, s.QuoteCurrency, s.SpreadPercent,
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
