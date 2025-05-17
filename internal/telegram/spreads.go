package telegram

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coinmarketcap" // ДОДАНО ІМПОРТ
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
)

// SpreadOpportunity ... (структура без змін)
type SpreadOpportunity struct {
	CoinID          string
	BaseCurrency    string
	QuoteCurrency   string
	BuyExchange     string
	BuyPriceUSD     float64
	SellExchange    string
	SellPriceUSD    float64
	SpreadPercent   float64
	ProfitPer100USD float64
	TrustScoreBuy   string
	TrustScoreSell  string
	TradeURLBuy     string
	TradeURLSell    string
	Comment         string
	Category        int
}

// isUserExchange ... (функція без змін)
func isUserExchange(exchangeIdentifier string, userExchanges []string) bool {
	normalizedIdentifier := strings.ToLower(strings.ReplaceAll(exchangeIdentifier, " ", "_"))
	for _, ue := range userExchanges {
		if normalizedIdentifier == ue {
			return true
		}
	}
	return false
}

// checkTrustScore ... (функція без змін)
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

// classifySpread ... (функція без змін)
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
	log.Printf("Спреди: Початок пошуку. Топ монет (CG): %d, Мін. спред: %.2f%%", cfg.SpreadCoinCount, cfg.SpreadMinPercentage)

	var allFoundSpreads []SpreadOpportunity
	userExchangesMap := make(map[string]bool)
	for _, ex := range cfg.SpreadUserExchanges {
		userExchangesMap[ex] = true
	}
	
	hitRateLimitCoinGecko := false

	// --- Етап 1: Отримання даних з CoinGecko ---
	log.Println("Спреди: Спроба отримати дані з CoinGecko...")
	topCoinsCG, errCG := coingecko.GetTopMarketCapCoins(cfg.SpreadCoinCount, "usd")
	if errCG != nil {
		log.Printf("Спреди: Помилка отримання топ монет з CoinGecko: %v", errCG)
		if strings.Contains(errCG.Error(), "429") {
			hitRateLimitCoinGecko = true
			log.Println("Спреди: Досягнуто ліміту CoinGecko при отриманні списку топ-монет.")
		}
		// Не виходимо, спробуємо CoinMarketCap
	} else {
		log.Printf("Спреди: Отримано %d монет для аналізу з CoinGecko.", len(topCoinsCG))
		processedCoinsCG := 0
		var mu sync.Mutex
		var wg sync.WaitGroup
		delayBetweenCoinProcessingCG := 6 * time.Second

		for i, coinLoopVar := range topCoinsCG {
			if hitRateLimitCoinGecko { break } // Якщо вже був ліміт на топ-монетах, не робимо запити тікерів
			wg.Add(1)
			go func(currentCoin coingecko.CoinMarketData) {
				defer wg.Done()
				mu.Lock()
				processedCoinsCG++
				currentProcessedLocal := processedCoinsCG
				mu.Unlock()

				if originalMessageID != 0 && (currentProcessedLocal%1 == 0 || currentProcessedLocal == len(topCoinsCG)) {
					progressText := fmt.Sprintf("⏳ CG: Оброблено %d/%d: %s...", currentProcessedLocal, len(topCoinsCG), currentCoin.Name)
					// Редагуємо повідомлення про прогрес (поки що одне загальне)
					if currentMessageID, ok := spreadProgressMessages[chatID]; ok && currentMessageID != 0 {
						editProgressMsg := tgbotapi.NewEditMessageText(chatID, currentMessageID, progressText)
						_, _ = bot.Send(editProgressMsg)
					}
				}
				
				tickersResponse, errTicker := coingecko.GetCoinTickers(currentCoin.ID, 1)
				if errTicker != nil {
					if strings.Contains(errTicker.Error(), "429") {
						log.Printf("Спреди: Досягнуто ліміту CoinGecko при отриманні тікерів для %s.", currentCoin.ID)
						mu.Lock() // Потрібно захистити доступ до hitRateLimitCoinGecko
						hitRateLimitCoinGecko = true
						mu.Unlock()
					}
					return
				}
				// ... (решта логіки обробки тікерів CoinGecko та пошуку спредів, як раніше) ...
				// Цю частину потрібно буде уніфікувати або дублювати для даних з CoinMarketCap
				var validTickers []coingecko.CoinGeckoTickerDetail
				for _, ticker := range tickersResponse.Tickers {
					if strings.ToUpper(ticker.Target) != "USDT" { continue }
					if !checkTrustScore(ticker.TrustScore, cfg.SpreadMinTrustScore) { continue }
					if priceUSD, ok := ticker.ConvertedLast["usd"]; ok && priceUSD > 0 {
						validTickers = append(validTickers, ticker)
					}
				}
				
				if len(validTickers) < 2 { return }

				for k := 0; k < len(validTickers); k++ {
					for l := k + 1; l < len(validTickers); l++ {
						// ... (розрахунок спреду) ...
						// op := SpreadOpportunity{...}
						// mu.Lock()
						// allFoundSpreads = append(allFoundSpreads, op)
						// mu.Unlock()
					}
				}
				// -----
				if !hitRateLimitCoinGecko { time.Sleep(delayBetweenCoinProcessingCG) }
			}(coinLoopVar)
			if hitRateLimitCoinGecko { break } // Якщо в одній з горутин спрацював ліміт, зупиняємо цикл
		}
		wg.Wait()
	}

	// --- Етап 2: Отримання даних з CoinMarketCap (якщо потрібно) ---
	if cfg.CoinMarketCapAPIKey != "" && (hitRateLimitCoinGecko || len(allFoundSpreads) < 5) { // Приклад умови: якщо CG дав мало або ліміт
		log.Println("Спреди: Спроба отримати дані з CoinMarketCap...")
		// Викликаємо функції з coinmarketcap.go
		topCoinsCMC, errCMC := coinmarketcap.GetTopMarketCapCoinsCMC(cfg.CoinMarketCapAPIKey, cfg.SpreadCoinCount)
		if errCMC != nil {
			log.Printf("Спреди: Помилка отримання топ монет з CoinMarketCap: %v", errCMC)
		} else {
			log.Printf("Спреди: Отримано %d монет для аналізу з CoinMarketCap.", len(topCoinsCMC))
			// Обробка монет з CMC аналогічно до CG
			for _, coinCMC := range topCoinsCMC {
				// Оновлення прогресу
				if originalMessageID != 0 {
					progressText := fmt.Sprintf("⏳ CMC: Обробка %s...", coinCMC.Name)
					if currentMessageID, ok := spreadProgressMessages[chatID]; ok && currentMessageID != 0 {
						editProgressMsg := tgbotapi.NewEditMessageText(chatID, currentMessageID, progressText)
						_, _ = bot.Send(editProgressMsg)
					}
				}

				tickersCMC, errTickersCMC := coinmarketcap.GetCoinTickersCMC(cfg.CoinMarketCapAPIKey, coinCMC.ID) // Використовуємо ID монети
				if errTickersCMC != nil {
					log.Printf("Спреди: Помилка отримання тікерів з CoinMarketCap для %s: %v", coinCMC.Name, errTickersCMC)
					if strings.Contains(errTickersCMC.Error(), "429") || (strings.Contains(errTickersCMC.Error(), "code 1008") && strings.Contains(errTickersCMC.Error(), "rate limit")) { // Приклад коду помилки ліміту CMC
						log.Println("Спреди: Досягнуто ліміту CoinMarketCap. Завершуємо пошук.")
						break
					}
					continue
				}
				log.Printf("Спреди: CoinMarketCap: отримано %d тікерів для %s", len(tickersCMC), coinCMC.Name)
				// Тут буде логіка перетворення UnifiedTickerInfoCMC в щось сумісне
				// і додавання до allFoundSpreads, якщо знайдені спреди.
				// Поки що просто логуємо.
				time.Sleep(2 * time.Second) // Затримка між запитами до CMC
			}
		}
	}


	// --- Формування та відправка звіту ---
	// (Поточний код сортування та відображення на основі allFoundSpreads)
	// ... (код сортування та відображення як у відповіді #327) ...
	sort.SliceStable(allFoundSpreads, func(i, j int) bool {
		if allFoundSpreads[i].Category != allFoundSpreads[j].Category {
			return allFoundSpreads[i].Category < allFoundSpreads[j].Category
		}
		return allFoundSpreads[i].SpreadPercent > allFoundSpreads[j].SpreadPercent
	})
	
	var reportText strings.Builder
	reportText.WriteString(fmt.Sprintf("📈 **Знайдені Спреди (мін. %.2f%%, Топ-%d монет):**\n", cfg.SpreadMinPercentage, cfg.SpreadCoinCount))
	reportText.WriteString("_Увага: Ціни з CoinGecko/CMC, можуть відрізнятися від реальних. Завжди перевіряйте на біржах! Комісії не враховані._\n\n")

	if len(allFoundSpreads) == 0 {
		reportText.WriteString("Спредів, що відповідають вашим критеріям, не знайдено.")
		if hitRateLimitCoinGecko { // Додаємо повідомлення, якщо був ліміт CG
			reportText.WriteString("\n\n⚠️ Було досягнуто ліміту запитів до CoinGecko. Результати можуть бути неповними.")
		}
		// Можна додати аналогічне повідомлення для ліміту CMC
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
		if hitRateLimitCoinGecko { 
			reportText.WriteString("\n\n⚠️ Було досягнуто ліміту запитів до CoinGecko. Показані результати можуть бути неповними.")
		}
	}
	
	finalText := reportText.String()
	if len(finalText) > MaxTelegramMessageSize {
		log.Printf("Спреди: Повідомлення занадто довге (%d). Обрізаємо.", len(finalText))
		finalText = finalText[:MaxTelegramMessageSize-30] + "\n... (повідомлення обрізано)"
	}

	// Видаляємо або редагуємо початкове повідомлення "Завантажую..."
    // Потрібно мати ID цього повідомлення. Зараз originalMessageID передається, але не використовується для редагування "loading"
    // Замість цього, HandleSpreadsCommand тепер сам надсилає "loading" і може його видалити.
	// Але в горутині ми не можемо безпечно видаляти/редагувати оригінальне повідомлення безпосередньо.
    // Тому, якщо HandleSpreadsCommand викликається в горутині, вона має надіслати новий фінальний звіт.
    // Якщо originalMessageID було передано з handler.go (ID повідомлення "Розпочато пошук..."), то його можна видалити.
	if originalMessageIDFromHandler, ok := spreadProgressMessages[chatID]; ok && originalMessageIDFromHandler != 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, originalMessageIDFromHandler)
		_, _ = bot.Send(deleteMsg)
		delete(spreadProgressMessages, chatID) // Очищаємо після використання
	}
	
	finalMsg := tgbotapi.NewMessage(chatID, finalText)
	finalMsg.ParseMode = tgbotapi.ModeMarkdown
	finalMsg.DisableWebPagePreview = true 
	sendAndLog(bot, finalMsg, "spreads_report_final", chatID)
	
	keyboard.ShowMainKeyboard(bot, chatID)
}

// Для зберігання ID повідомлень про прогрес для кожного чату
var spreadProgressMessages = make(map[int64]int)
var spreadProgressMutex sync.Mutex

func min(a, b int) int {
	if a < b { return a }
	return b
}
