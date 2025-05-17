package telegram

import (
	"fmt"
	"log"
	"sort"
	"strconv" // Потрібен для strconv.Atoi в checkTrustScore, якщо будемо парсити числові Trust Scores
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
func classifySpread(coinSymbol string, exchangeBuyID, exchangeSellID string, userExchangesMap map[string]bool, allCoinGeckoTickers []coingecko.CoinGeckoTickerDetail) (string, int) { // Поки що приймає CoinGeckoTickerDetail
	buyIsUser := userExchangesMap[strings.ToLower(exchangeBuyID)]
	sellIsUser := userExchangesMap[strings.ToLower(exchangeSellID)]

	tokenOnUserOtherExchange := false
	var userExchangeWithToken string
	// Цю логіку потрібно буде адаптувати, якщо ми об'єднуємо тікери з різних джерел
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
	log.Printf("Спреди: Початок пошуку. Топ монет: %d, Мін. спред: %.2f%%", cfg.SpreadCoinCount, cfg.SpreadMinPercentage)

	var allFoundSpreads []SpreadOpportunity
	userExchangesMap := make(map[string]bool)
	for _, ex := range cfg.SpreadUserExchanges {
		userExchangesMap[ex] = true
	}

	// --- Отримання даних з CoinGecko ---
	log.Println("Спреди: Спроба отримати дані з CoinGecko...")
	topCoinsCG, errCG := coingecko.GetTopMarketCapCoins(cfg.SpreadCoinCount, "usd")
	if errCG != nil {
		log.Printf("Спреди: Помилка отримання топ монет з CoinGecko: %v", errCG)
		// Не виходимо, спробуємо CoinMarketCap
	} else {
		log.Printf("Спреди: Отримано %d монет для аналізу з CoinGecko.", len(topCoinsCG))
		// Тут буде логіка обробки даних з CoinGecko, як раніше (з горутинами і затримками)
		// Поки що просто залогуємо, що отримали
	}

	// --- Отримання даних з CoinMarketCap ---
	if cfg.CoinMarketCapAPIKey != "" {
		log.Println("Спреди: Спроба отримати дані з CoinMarketCap...")
		topCoinsCMC, errCMC := coinmarketcap.GetTopMarketCapCoinsCMC(cfg.CoinMarketCapAPIKey, cfg.SpreadCoinCount)
		if errCMC != nil {
			log.Printf("Спреди: Помилка отримання топ монет з CoinMarketCap: %v", errCMC)
		} else {
			log.Printf("Спреди: Отримано %d монет для аналізу з CoinMarketCap.", len(topCoinsCMC))
			for _, coinCMC := range topCoinsCMC {
				log.Printf("Спреди (CMC): Обробка %s (%s)", coinCMC.Name, coinCMC.Symbol)
				// Тут буде запит тікерів з coinmarketcap.GetCoinTickersCMC
				// та додавання знайдених спредів до allFoundSpreads
				// Потрібно буде уніфікувати дані з UnifiedTickerInfoCMC до SpreadOpportunity
				// або адаптувати classifySpread
				// Наприклад:
				// cmcTickers, errCMCTickers := coinmarketcap.GetCoinTickersCMC(cfg.CoinMarketCapAPIKey, coinCMC.ID)
				// if errCMCTickers != nil {
				// log.Printf("Спреди (CMC): Помилка отримання тікерів для %s: %v", coinCMC.Name, errCMCTickers)
				// continue
				// }
				// log.Printf("Спреди (CMC): Для %s отримано %d тікерів.", coinCMC.Name, len(cmcTickers))
				// // Далі логіка пошуку спредів на основі cmcTickers
			}
		}
	} else {
		log.Println("Спреди: API ключ для CoinMarketCap не надано, пропускаємо.")
	}

	// --- ПОКИ ЩО ФОРМУЄМО ЗВІТ НА ОСНОВІ ЗАГЛУШКИ ---
	// Потрібно буде об'єднати allFoundSpreads з обох джерел, якщо ми їх використовуємо
	
	sort.SliceStable(allFoundSpreads, func(i, j int) bool {
		if allFoundSpreads[i].Category != allFoundSpreads[j].Category {
			return allFoundSpreads[i].Category < allFoundSpreads[j].Category
		}
		return allFoundSpreads[i].SpreadPercent > allFoundSpreads[j].SpreadPercent
	})
	
	var reportText strings.Builder
	reportText.WriteString(fmt.Sprintf("📈 **Знайдені Спреди (мін. %.2f%%, Топ-%d монет):**\n", cfg.SpreadMinPercentage, cfg.SpreadCoinCount))
	reportText.WriteString("_Увага: Ціни з агрегаторів, можуть відрізнятися від реальних. Завжди перевіряйте на біржах! Комісії не враховані._\n\n")

	if len(allFoundSpreads) == 0 {
		reportText.WriteString("Спредів, що відповідають вашим критеріям, не знайдено.")
		if errCG != nil && strings.Contains(errCG.Error(), "429") {
			reportText.WriteString("\n\n⚠️ Було досягнуто ліміту запитів до CoinGecko.")
		}
		// Додати аналогічне повідомлення для CoinMarketCap, якщо потрібно
	} else {
		// ... (код відображення знайдених спредів, як раніше) ...
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
