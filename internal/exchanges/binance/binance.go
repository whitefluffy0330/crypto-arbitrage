package binance

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

const (
	binanceFuturesAPIEndpoint = "https://fapi.binance.com"
	premiumIndexPath        = "/fapi/v1/premiumIndex" // Ендпоінт для Mark Price та Estimated Funding Rate
)

// BinancePremiumIndex відповідає структурі відповіді з /fapi/v1/premiumIndex
type BinancePremiumIndex struct {
	Symbol               string `json:"symbol"`
	MarkPrice            string `json:"markPrice"`            // Поточна Mark Price
	IndexPrice           string `json:"indexPrice"`           // Ціна індексу
	EstimatedSettlePrice string `json:"estimatedSettlePrice"` // Оціночна ціна розрахунку (для деяких контрактів)
	LastFundingRate      string `json:"lastFundingRate"`      // Остання ставка фінансування, що спрацювала
	NextFundingTime      int64  `json:"nextFundingTime"`      // Час наступного фінансування (мілісекунди Unix)
	InterestRate         string `json:"interestRate"`         // Поточна процентна ставка (зазвичай для маржі)
	Time                 int64  `json:"time"`                 // Час відповіді сервера (мілісекунди Unix)
}

// FundingInfo - простіша структура для повернення корисних даних
type FundingInfo struct {
	Symbol          string
	MarkPrice       float64
	LastFundingRate float64   // Остання ставка фінансування
	NextFundingTime time.Time // Час наступного фінансування
}

// GetFundingRates отримує Mark Price та Funding Rate для всіх символів з Binance Futures
// і повертає мапу map[Symbol]FundingInfo.
func GetFundingRates() (map[string]FundingInfo, error) {
	url := binanceFuturesAPIEndpoint + premiumIndexPath
	log.Printf("Запит до Binance API: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Помилка запиту до Binance API (%s): %v", url, err)
		return nil, fmt.Errorf("помилка HTTP запиту до Binance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Помилка статусу від Binance API (%s): %s", url, resp.Status)
		// Тут можна прочитати тіло відповіді для деталей помилки, якщо потрібно
		return nil, fmt.Errorf("помилка статусу від Binance API: %s", resp.Status)
	}

	var results []BinancePremiumIndex // Очікуємо масив об'єктів
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&results)
	if err != nil {
		// Якщо відповідь - це один об'єкт, а не масив (можливо, якщо запитати 1 символ)
		// Спробуємо розпарсити як один об'єкт (хоча ми запитуємо всі)
		var singleResult BinancePremiumIndex
		// Потрібно буде перезавантажити тіло відповіді, якщо це можливо, або зробити новий запит
		// Простіше поки що обробляти лише масив
		log.Printf("Помилка декодування JSON масиву від Binance API: %v", err)
		return nil, fmt.Errorf("помилка розбору відповіді від Binance: %w", err)
	}

	fundingData := make(map[string]FundingInfo)
	for _, item := range results {
		markPrice, _ := strconv.ParseFloat(item.MarkPrice, 64)
		lastFundingRate, _ := strconv.ParseFloat(item.LastFundingRate, 64)
		// Конвертуємо мілісекунди Unix в time.Time
		nextFundingTime := time.Unix(0, item.NextFundingTime*int64(time.Millisecond))

		fundingData[item.Symbol] = FundingInfo{
			Symbol:          item.Symbol,
			MarkPrice:       markPrice,
			LastFundingRate: lastFundingRate * 100, // Переводимо у відсотки для зручності
			NextFundingTime: nextFundingTime,
		}
	}

	log.Printf("Отримано дані фінансування для %d символів з Binance.", len(fundingData))
	return fundingData, nil
}