package telegram

import (
	"fmt" // Залишимо для fmt.Sprintf у майбутньому
	"log"
	// "sort"    // Тимчасово не потрібен
	// "strconv" // Тимчасово не потрібен
	// "strings" // Тимчасово не потрібен
	// "sync"    // Тимчасово не потрібен
	"time" // Залишимо для time.Sleep у майбутньому

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	// Ключовий імпорт, який ми тестуємо:
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko"
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard" // Поки не використовується
)

// SpreadOpportunity ... (залишаємо структуру, вона не має викликати помилок)
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

// isUserExchange ... (залишаємо, не має викликати помилок)
func isUserExchange(exchangeIdentifier string, userExchanges []string) bool {
	normalizedIdentifier := strings.ToLower(strings.ReplaceAll(exchangeIdentifier, " ", "_"))
	for _, ue := range userExchanges {
		if normalizedIdentifier == ue {
			return true
		}
	}
	return false
}
// checkTrustScore ... (залишаємо, не має викликати помилок)
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
// classifySpread ... (залишаємо, не має викликати помилок)
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
	var testVar coingecko.CoinMarketData // Оголошуємо змінну типу coingecko.CoinMarketData
	log.Printf("Спреди: Тестове оголошення coingecko.CoinMarketData.ID: %s (це для діагностики)", testVar.ID)
	// ---- КІНЕЦЬ ДІАГНОСТИКИ ----

	// Поки що весь інший код функції закоментовано для ізоляції проблеми
	/*
		loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Пошук спредів... Це може зайняти деякий час, будь ласка, зачекайте.")
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
			// ... (обробка помилки) ...
			return
		}
		// ... (решта логіки) ...
	*/

	// Поки що просто надсилаємо повідомлення, що функція викликана
	diagnosticMsg := tgbotapi.NewMessage(chatID, "Функція HandleSpreadsCommand викликана. Діагностика типу coingecko.CoinMarketData виконана (дивіться логи сервера).")
	sendAndLog(bot, diagnosticMsg, "spreads_command_called_diag", chatID)
	// keyboard.ShowMainKeyboard(bot, chatID) // Поки що не показуємо, щоб не заважати
}

// min функція не використовується, якщо не використовується логіка вище
// func min(a, b int) int {
// 	if a < b { return a }
// 	return b
// }
