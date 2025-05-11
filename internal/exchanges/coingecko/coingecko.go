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
)

// CoinMarketData містить основні дані про монету з ендпоінту /coins/markets
type CoinMarketData struct {
	ID             string  `json:"id"`               // Наприклад, "bitcoin"
	Symbol         string  `json:"symbol"`           // Наприклад, "btc"
	Name           string  `json:"name"`             // Наприклад, "Bitcoin"
	Image          string  `json:"image"`            // URL зображення
	CurrentPrice   float64 `json:"current_price"`    // Поточна ціна
	MarketCap      int64   `json:"market_cap"`       // Ринкова капіталізація
	MarketCapRank  int     `json:"market_cap_rank"`  // Ранг за капіталізацією
	TotalVolume    float64 `json:"total_volume"`     // Загальний об'єм торгів
	High24h        float64 `json:"high_24h"`         // Максимум за 24 години
	Low24h         float64 `json:"low_24h"`          // Мінімум за 24 години
	PriceChange24h float64 `json:"price_change_24h"` // Зміна ціни за 24 години
}

// GetTopMarketCapCoins отримує список топ-N монет за ринковою капіталізацією.
// vsCurrency - валюта, до якої порівнюється капіталізація (наприклад, "usd", "eur").
// limit - кількість монет у списку.
func GetTopMarketCapCoins(limit int, vsCurrency string) ([]CoinMarketData, error) {
	if limit <= 0 {
		limit = 100 // Значення за замовчуванням
	}
	if vsCurrency == "" {
		vsCurrency = "usd" // Валюта за замовчуванням
	}

	// Формуємо URL запиту
	//Приклад: https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&order=market_cap_desc&per_page=100&page=1&sparkline=false
	url := fmt.Sprintf("%s%s?vs_currency=%s&order=market_cap_desc&per_page=%d&page=1&sparkline=false",
		coinGeckoAPIEndpoint,
		coinsMarketsPath,
		vsCurrency,
		limit,
	)

	log.Printf("Запит до CoinGecko API: %s", url)

	// Створюємо HTTP клієнт з таймаутом
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Помилка HTTP запиту до CoinGecko API (%s): %v", url, err)
		return nil, fmt.Errorf("помилка HTTP запиту до CoinGecko: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Помилка статусу від CoinGecko API (%s): %s", url, resp.Status)
		// Тут можна прочитати тіло відповіді для деталей помилки, якщо потрібно
		return nil, fmt.Errorf("помилка статусу від CoinGecko API: %s", resp.Status)
	}

	var results []CoinMarketData // Очікуємо масив об'єктів
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&results)
	if err != nil {
		log.Printf("Помилка декодування JSON відповіді від CoinGecko API: %v", err)
		return nil, fmt.Errorf("помилка розбору відповіді від CoinGecko: %w", err)
	}

	log.Printf("Отримано топ-%d монет з CoinGecko.", len(results))
	return results, nil
}
