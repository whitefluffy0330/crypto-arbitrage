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
	premiumIndexPath        = "/fapi/v1/premiumIndex" 
)

type BinancePremiumIndex struct {
	Symbol               string `json:"symbol"`
	MarkPrice            string `json:"markPrice"`            
	IndexPrice           string `json:"indexPrice"`           
	EstimatedSettlePrice string `json:"estimatedSettlePrice"` 
	LastFundingRate      string `json:"lastFundingRate"`      
	NextFundingTime      int64  `json:"nextFundingTime"`      
	InterestRate         string `json:"interestRate"`         
	Time                 int64  `json:"time"`                 
}

type FundingInfo struct {
	Symbol          string
	MarkPrice       float64
	LastFundingRate float64   
	NextFundingTime time.Time 
}

// GetFundingRates отримує Mark Price та Funding Rate для всіх символів з Binance Futures
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
		return nil, fmt.Errorf("помилка статусу від Binance API: %s", resp.Status)
	}

	var results []BinancePremiumIndex 
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&results) 
	if err != nil {
		// var singleResult BinancePremiumIndex // <<< ВИДАЛЕНО ЦЕЙ РЯДОК
		// Якщо декодування масиву не вдалося, повертаємо помилку
		log.Printf("Помилка декодування JSON масиву від Binance API: %v", err)
		return nil, fmt.Errorf("помилка розбору відповіді від Binance: %w", err)
	}

	fundingData := make(map[string]FundingInfo)
	for _, item := range results {
		markPrice, _ := strconv.ParseFloat(item.MarkPrice, 64)
		lastFundingRate, _ := strconv.ParseFloat(item.LastFundingRate, 64)
		nextFundingTime := time.Unix(0, item.NextFundingTime*int64(time.Millisecond))

		fundingData[item.Symbol] = FundingInfo{
			Symbol:          item.Symbol,
			MarkPrice:       markPrice,
			LastFundingRate: lastFundingRate * 100, // У відсотках
			NextFundingTime: nextFundingTime,
		}
	}

	log.Printf("Отримано дані фінансування для %d символів з Binance.", len(fundingData))
	return fundingData, nil
}
