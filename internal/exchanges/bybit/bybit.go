package bybit

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
	bybitAPIEndpoint = "https://api.bybit.com"
	tickersPathV5    = "/v5/market/tickers"
)

// BybitTickerV5 містить поля, які нас цікавлять з відповіді API /v5/market/tickers
type BybitTickerV5 struct {
	Symbol          string `json:"symbol"`
	MarkPrice       string `json:"markPrice"`
	FundingRate     string `json:"fundingRate"`     // Ставка фінансування
	NextFundingTime string `json:"nextFundingTime"` // Час наступного фінансування (мілісекунди)
	// Додамо інші поля, якщо вони знадобляться, наприклад, IndexPrice
}

// BybitTickersResponseV5 структура для загальної відповіді API
type BybitTickersResponseV5 struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Category string          `json:"category"`
		List     []BybitTickerV5 `json:"list"`
	} `json:"result"`
	RetExtInfo interface{} `json:"retExtInfo"` // Може бути {}, може бути інше
	Time       int64       `json:"time"`
}

// GetFundingRates отримує ставки фінансування для USDT-M контрактів з Bybit
func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	// Ми хочемо дані для USDT Perpetual, це категорія "linear"
	url := fmt.Sprintf("%s%s?category=linear", bybitAPIEndpoint, tickersPathV5)
	log.Printf("Запит до Bybit API для Funding Rates: %s", url)

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Помилка HTTP GET запиту до Bybit API (%s): %v", url, err)
		return nil, fmt.Errorf("помилка HTTP GET запиту до Bybit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Помилка статусу від Bybit API (%s): %s", url, resp.Status)
		// Можна спробувати прочитати тіло відповіді для деталей помилки
		// bodyBytes, _ := io.ReadAll(resp.Body)
		// log.Printf("Тіло відповіді Bybit: %s", string(bodyBytes))
		return nil, fmt.Errorf("помилка статусу від Bybit API: %s (%d)", resp.Status, resp.StatusCode)
	}

	var apiResponse BybitTickersResponseV5
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&apiResponse); err != nil {
		log.Printf("Помилка декодування JSON відповіді від Bybit API: %v", err)
		return nil, fmt.Errorf("помилка розбору JSON відповіді від Bybit: %w", err)
	}

	if apiResponse.RetCode != 0 {
		log.Printf("API Bybit повернуло помилку: Code=%d, Msg=%s", apiResponse.RetCode, apiResponse.RetMsg)
		return nil, fmt.Errorf("API Bybit повернуло помилку: %s (код %d)", apiResponse.RetMsg, apiResponse.RetCode)
	}

	var fundingData []exchanges.UnifiedFundingRateInfo
	for _, item := range apiResponse.Result.List {
		// Переконуємося, що це USDT Perpetual
		if !strings.HasSuffix(item.Symbol, "USDT") {
			continue
		}
		// Пропускаємо, якщо поля порожні (може бути для деяких неактивних контрактів)
		if item.MarkPrice == "" || item.FundingRate == "" || item.NextFundingTime == "" {
			// log.Printf("Пропуск символу %s через порожні поля (MarkPrice: '%s', FundingRate: '%s', NextFundingTime: '%s')", item.Symbol, item.MarkPrice, item.FundingRate, item.NextFundingTime)
			continue
		}

		markPrice, errMP := strconv.ParseFloat(item.MarkPrice, 64)
		if errMP != nil {
			log.Printf("Bybit: Помилка парсингу MarkPrice для %s ('%s'): %v. Пропускаємо.", item.Symbol, item.MarkPrice, errMP)
			continue
		}

		// FundingRate на Bybit також йде як десяткове число (0.0001 для 0.01%)
		fundingRateRaw, errFR := strconv.ParseFloat(item.FundingRate, 64)
		if errFR != nil {
			log.Printf("Bybit: Помилка парсингу FundingRate для %s ('%s'): %v. Пропускаємо.", item.Symbol, item.FundingRate, errFR)
			continue
		}
		fundingRatePercent := fundingRateRaw * 100 // Конвертуємо у відсотки

		nextFundingTimeMs, errNFT := strconv.ParseInt(item.NextFundingTime, 10, 64)
		if errNFT != nil {
			log.Printf("Bybit: Помилка парсингу NextFundingTime для %s ('%s'): %v. Пропускаємо.", item.Symbol, item.NextFundingTime, errNFT)
			continue
		}
		nextFundingTime := time.Unix(0, nextFundingTimeMs*int64(time.Millisecond)).UTC()

		fundingData = append(fundingData, exchanges.UnifiedFundingRateInfo{
			Exchange:        "Bybit",
			Symbol:          item.Symbol,
			MarkPrice:       markPrice,
			LastFundingRate: fundingRatePercent,
			NextFundingTime: nextFundingTime,
		})
	}

	log.Printf("Отримано та оброблено дані фінансування для %d USDT пар з Bybit.", len(fundingData))
	return fundingData, nil
}
