package binance

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings" // Додано для HasSuffix
	"time"

	// Імпортуємо наш новий пакет exchanges для UnifiedFundingRateInfo
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges"
)

const (
	binanceFuturesAPIEndpoint = "https://fapi.binance.com"
	premiumIndexPath        = "/fapi/v1/premiumIndex"
)

// Структура для парсингу відповіді API Binance
type BinancePremiumIndexAPIResponse struct {
	Symbol               string `json:"symbol"`
	MarkPrice            string `json:"markPrice"`
	IndexPrice           string `json:"indexPrice"`
	EstimatedSettlePrice string `json:"estimatedSettlePrice"`
	LastFundingRate      string `json:"lastFundingRate"`
	NextFundingTime      int64  `json:"nextFundingTime"`
	InterestRate         string `json:"interestRate"`
	Time                 int64  `json:"time"`
}

// GetFundingRates тепер повертає []exchanges.UnifiedFundingRateInfo
func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	url := binanceFuturesAPIEndpoint + premiumIndexPath
	log.Printf("Запит до Binance API для Funding Rates: %s", url)

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Помилка HTTP GET запиту до Binance API (%s): %v", url, err)
		return nil, fmt.Errorf("помилка HTTP GET запиту до Binance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Помилка статусу від Binance API (%s): %s", url, resp.Status)
		return nil, fmt.Errorf("помилка статусу від Binance API: %s", resp.Status)
	}

	var apiResults []BinancePremiumIndexAPIResponse // Для парсингу JSON відповіді
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&apiResults); err != nil {
		log.Printf("Помилка декодування JSON відповіді від Binance API: %v", err)
		return nil, fmt.Errorf("помилка розбору JSON відповіді від Binance: %w", err)
	}

	var fundingData []exchanges.UnifiedFundingRateInfo // Результат у новому уніфікованому форматі
	for _, item := range apiResults {
		// Відфільтровуємо тільки USDT-M ф'ючерси (безстрокові)
		// Можна додати більш складну логіку, якщо потрібно (наприклад, COIN-M або конкретні типи)
		if !strings.HasSuffix(item.Symbol, "USDT") {
			continue // Пропускаємо пари не з USDT, наприклад, BUSD або квартальні
		}
		// Можна додати перевірку, чи є символ числом (деякі старі контракти типу 1000SHIBUSDT)
		// Тут можна також відфільтрувати за обсягом торгів, якщо API дозволяє або якщо ми маємо ці дані

		markPrice, errMP := strconv.ParseFloat(item.MarkPrice, 64)
		if errMP != nil {
			log.Printf("Помилка парсингу MarkPrice для %s: %v. Пропускаємо.", item.Symbol, errMP)
			continue
		}

		lastFundingRateRaw, errLFR := strconv.ParseFloat(item.LastFundingRate, 64)
		if errLFR != nil {
			log.Printf("Помилка парсингу LastFundingRate для %s: %v. Пропускаємо.", item.Symbol, errLFR)
			continue
		}
		// Binance API повертає ставку як десяткове число (напр. 0.0001 для 0.01%)
		// Зберігаємо її у відсотках (напр. 0.01 для 0.01%)
		lastFundingRatePercent := lastFundingRateRaw * 100

		nextFundingTime := time.Unix(0, item.NextFundingTime*int64(time.Millisecond)).UTC()

		fundingData = append(fundingData, exchanges.UnifiedFundingRateInfo{
			Exchange:        "Binance", // Вказуємо назву біржі
			Symbol:          item.Symbol,
			MarkPrice:       markPrice,
			LastFundingRate: lastFundingRatePercent,
			NextFundingTime: nextFundingTime,
		})
	}

	log.Printf("Отримано та оброблено дані фінансування для %d USDT пар з Binance.", len(fundingData))
	return fundingData, nil
}
