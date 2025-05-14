package coingecko

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	coinGeckoAPIEndpoint = "https://api.coingecko.com/api/v3"
	coinsMarketsPath     = "/coins/markets"
	coinTickersPath      = "/coins/%s/tickers" // %s буде замінено на coin_id
)

type MarketCoin struct {
	ID            string  `json:"id"`
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	CurrentPrice  float64 `json:"current_price"`
	MarketCap     float64 `json:"market_cap"`
	MarketCapRank int     `json:"market_cap_rank"`
	Image         string  `json:"image"`
}

// CoinGeckoTickerDetail структура для одного тікера з відповіді /coins/{id}/tickers
type CoinGeckoTickerDetail struct {
	Base   string `json:"base"`   // Наприклад, "BTC"
	Target string `json:"target"` // Наприклад, "USDT"
	Market struct {
		Name                string `json:"name"`
		Identifier          string `json:"identifier"`
		HasTradingIncentive bool   `json:"has_trading_incentive"`
	} `json:"market"`
	Last                   float64     `json:"last"`
	Volume                 float64     `json:"volume"` // Обсяг в базовій валюті (кількість монет)
	ConvertedLast          map[string]float64 `json:"converted_last"` // {"btc": ..., "eth": ..., "usd": ...}
	ConvertedVolume        map[string]float64 `json:"converted_volume"`
	TrustScore             string      `json:"trust_score"` // "green", "yellow", "red"
	BidAskSpreadPercentage float64     `json:"bid_ask_spread_percentage"`
	Timestamp              time.Time   `json:"timestamp"`
	LastTradedAt           time.Time   `json:"last_traded_at"`
	LastFetchAt            time.Time   `json:"last_fetch_at"`
	IsAnomaly              bool        `json:"is_anomaly"`
	IsStale                bool        `json:"is_stale"`
	TradeURL               string      `json:"trade_url"`
	TokenInfoURL           interface{} `json:"token_info_url"` // Може бути null
	CoinID                 string      `json:"coin_id"`
	TargetCoinID           string      `json:"target_coin_id,omitempty"`
}

// CoinGeckoTickersResponse структура для відповіді /coins/{id}/tickers
type CoinGeckoTickersResponse struct {
	Name    string                  `json:"name"`
	Tickers []CoinGeckoTickerDetail `json:"tickers"`
}


func GetTopMarketCapCoins(limit int, vsCurrency string) ([]MarketCoin, error) {
	url := fmt.Sprintf("%s%s?vs_currency=%s&order=market_cap_desc&per_page=%d&page=1&sparkline=false",
		coinGeckoAPIEndpoint, coinsMarketsPath, vsCurrency, limit)

	log.Printf("CoinGecko: Запит топ монет: %s", url)
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("помилка HTTP запиту до CoinGecko: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("помилка статусу від CoinGecko API: %s", resp.Status)
	}

	var coins []MarketCoin
	if err := json.NewDecoder(resp.Body).Decode(&coins); err != nil {
		return nil, fmt.Errorf("помилка розбору JSON відповіді від CoinGecko: %w", err)
	}
	return coins, nil
}

// GetCoinTickers отримує тікери для конкретної монети з CoinGecko
func GetCoinTickers(coinID string, page int) (CoinGeckoTickersResponse, error) {
	// Додаємо include_exchange_logo=false&depth=false для зменшення обсягу відповіді
	// order=trust_score_desc (або volume_desc) для сортування, але може не бути критичним, якщо ми все одно фільтруємо
	url := fmt.Sprintf("%s%s?page=%d&include_exchange_logo=false&depth=false&order=volume_desc", 
		coinGeckoAPIEndpoint, fmt.Sprintf(coinTickersPath, coinID), page)

	log.Printf("CoinGecko: Запит тікерів для %s (сторінка %d): %s", coinID, page, url)
	var emptyResponse CoinGeckoTickersResponse // Повертаємо порожню структуру при помилці

	client := http.Client{Timeout: 15 * time.Second} // Трохи більший таймаут для тікерів
	resp, err := client.Get(url)
	if err != nil {
		return emptyResponse, fmt.Errorf("помилка HTTP запиту до CoinGecko (%s/tickers): %w", coinID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Спробуємо прочитати тіло помилки
		// bodyBytes, _ := io.ReadAll(resp.Body)
		// log.Printf("CoinGecko (%s/tickers): тіло відповіді при помилці статусу: %s", coinID, string(bodyBytes))
		return emptyResponse, fmt.Errorf("помилка статусу %d від CoinGecko API (%s/tickers)", resp.StatusCode, coinID)
	}

	var tickersResponse CoinGeckoTickersResponse
	if err := json.NewDecoder(resp.Body).Decode(&tickersResponse); err != nil {
		return emptyResponse, fmt.Errorf("помилка розбору JSON відповіді від CoinGecko (%s/tickers): %w", coinID, err)
	}
	return tickersResponse, nil
}
