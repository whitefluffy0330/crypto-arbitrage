package telegram

import (
	"fmt"
	"log"
	// "sort"    // Тимчасово видалено
	"strings"
	// "sync"    // Тимчасово видалено
	// "time" // Тимчасово видалено

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko"
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard" // Тимчасово видалено
)

// SpreadOpportunity ... (без змін)
type SpreadOpportunity struct {
	CoinID         string
	BaseCurrency   string
	QuoteCurrency  string
	BuyExchange    string
	BuyPriceUSD    float64
	SellExchange   string
	SellPriceUSD   float64
	SpreadPercent  float64
	TrustScoreBuy  string
	TrustScoreSell string
	TradeURLBuy    string
	TradeURLSell   string
	Comment        string
	Category       int
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
		log.Printf("Спреди: Невідомий формат SpreadMinTrustScore: '%s'. Фільтр не застосовано.", minTrustScoreConfig)
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
	// ---- ДІАГНОСТИКА ТИПУ ----
	var testVar []coingecko.CoinMarketData // Тепер використовуємо правильний тип CoinMarketData
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
	
	diagnosticMsg := tgbotapi.NewMessage(chatID, "Функція HandleSpreadsCommand викликана. Діагностика типу coingecko.CoinMarketData виконана (дивіться логи сервера).")
	sendAndLog(bot, diagnosticMsg, "spreads_command_called_diag", chatID)
	// keyboard.ShowMainKeyboard(bot, chatID) // Поки не показуємо
}

// min функція не використовується, якщо не використовується логіка вище
// func min(a, b int) int {
// 	if a < b { return a }
// 	return b
// }
