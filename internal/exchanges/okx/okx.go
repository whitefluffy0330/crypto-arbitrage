package okx

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges" // Для UnifiedFundingRateInfo
)

const (
	okxAPIEndpoint = "https://www.okx.com" // Базовий URL OKX
	tickersPathV5  = "/api/v5/market/tickers"
)

// OKXTickerV5 містить поля, які нас цікавлять з відповіді API /v5/market/tickers
// Поля можуть відрізнятися від Bybit, потрібно дивитися документацію OKX
type OKXTickerV5 struct {
	InstType        string `json:"instType"`        // Тип інструменту, наприклад "SWAP"
	InstID          string `json:"instId"`          // ID інструменту, наприклад "BTC-USDT-SWAP"
	MarkPx          string `json:"markPx"`          // Ціна маркування
	FundingRate     string `json:"fundingRate"`     // Ставка фінансування
	NextFundingTime string `json:"nextFundingTime"` // Час наступного фінансування (UTC мілісекунди)
	// Можуть бути й інші корисні поля, такі як 'vol24h', 'openInt'
}

// OKXAPIResponseV5 структура для загальної відповіді API (якщо вона є)
// OKX часто повертає масив даних у полі "data" і код помилки в "code"
type OKXAPIResponseV5 struct {
	Code string        `json:"code"` // "0" означає успіх
	Msg  string        `json:"msg"`
	Data []OKXTickerV5 `json:"data"`
}

// GetFundingRates отримує ставки фінансування для USDT-M SWAP контрактів з OKX
func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	// Ми хочемо дані для SWAP (безстрокові свопи)
	url := fmt.Sprintf("%s%s?instType=SWAP", okxAPIEndpoint, tickersPathV5)
	log.Printf("Запит до OKX API для Funding Rates: %s", url)

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Помилка HTTP GET запиту до OKX API (%s): %v", url, err)
		return nil, fmt.Errorf("помилка HTTP GET запиту до OKX: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Помилка статусу від OKX API (%s): %s", url, resp.Status)
		return nil, fmt.Errorf("помилка статусу від OKX API: %s (%d)", resp.Status, resp.StatusCode)
	}

	var apiResponse OKXAPIResponseV5
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&apiResponse); err != nil {
		log.Printf("Помилка декодування JSON відповіді від OKX API: %v", err)
		return nil, fmt.Errorf("помилка розбору JSON відповіді від OKX: %w", err)
	}

	if apiResponse.Code != "0" {
		log.Printf("API OKX повернуло помилку: Code=%s, Msg=%s", apiResponse.Code, apiResponse.Msg)
		return nil, fmt.Errorf("API OKX повернуло помилку: %s (код %s)", apiResponse.Msg, apiResponse.Code)
	}

	var fundingData []exchanges.UnifiedFundingRateInfo
	for _, item := range apiResponse.Data {
		// Переконуємося, що це USDT Perpetual (instId зазвичай закінчується на -USDT-SWAP)
		if !strings.HasSuffix(item.InstID, "-USDT-SWAP") {
			continue
		}
		// Пропускаємо, якщо важливі поля порожні
		if item.MarkPx == "" || item.FundingRate == "" || item.NextFundingTime == "" {
			continue
		}

		markPrice, errMP := strconv.ParseFloat(item.MarkPx, 64)
		if errMP != nil {
			log.Printf("OKX: Помилка парсингу MarkPx для %s ('%s'): %v. Пропускаємо.", item.InstID, item.MarkPx, errMP)
			continue
		}

		fundingRateRaw, errFR := strconv.ParseFloat(item.FundingRate, 64)
		if errFR != nil {
			log.Printf("OKX: Помилка парсингу FundingRate для %s ('%s'): %v. Пропускаємо.", item.InstID, item.FundingRate, errFR)
			continue
		}
		fundingRatePercent := fundingRateRaw * 100 // Конвертуємо у відсотки

		nextFundingTimeMs, errNFT := strconv.ParseInt(item.NextFundingTime, 10, 64)
		if errNFT != nil {
			log.Printf("OKX: Помилка парсингу NextFundingTime для %s ('%s'): %v. Пропускаємо.", item.InstID, item.NextFundingTime, errNFT)
			continue
		}
		nextFundingTime := time.Unix(0, nextFundingTimeMs*int64(time.Millisecond)).UTC()

		// Формуємо "чистий" символ, наприклад BTCUSDT з BTC-USDT-SWAP
		symbolClean := strings.Replace(item.InstID, "-SWAP", "", 1)
		symbolClean = strings.Replace(symbolClean, "-", "", 1)

		fundingData = append(fundingData, exchanges.UnifiedFundingRateInfo{
			Exchange:        "OKX",
			Symbol:          symbolClean, // Використовуємо "чистий" символ
			MarkPrice:       markPrice,
			LastFundingRate: fundingRatePercent,
			NextFundingTime: nextFundingTime,
		})
	}

	log.Printf("Отримано та оброблено дані фінансування для %d USDT SWAP пар з OKX.", len(fundingData))
	return fundingData, nil
}
