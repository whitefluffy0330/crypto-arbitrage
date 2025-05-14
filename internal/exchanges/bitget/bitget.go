package bitget

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges"
)

const (
	bitgetAPIEndpointBase = "https://api.bitget.com"
	tickersPathV2         = "/api/v2/mix/market/tickers"
	fundingTimePathV2     = "/api/v2/mix/market/funding-time" // Новий ендпоінт
	productTypeUSDT       = "USDT-FUTURES"                  // Або "umcbl" або інший ідентифікатор для USDT Perpetual
	topNByVolume          = 75
	maxConcurrentRequests = 5 // Для запитів funding-time
	maxErrorLogs          = 5
)

type BitgetTickerV2 struct {
	Symbol      string `json:"symbol"`
	MarkPrice   string `json:"markPrice"`
	FundingRate string `json:"fundingRate"`
	// NextFundingTime string `json:"nextFundingTime"` // Це поле, схоже, не завжди є або порожнє в /tickers
	QuoteVolume string `json:"quoteVolume"`
	LastPrice   string `json:"lastPr"`
	IndexPrice  string `json:"indexPrice"`
}

type BitgetFundingTimeResponseData struct {
	Symbol          string `json:"symbol"`
	NextFundingTime string `json:"nextFundingTime"` // Час наступного розрахунку (ms)
	RatePeriod      string `json:"ratePeriod"`      // Період ставки (години)
}

type BitgetAPIResponseV2 struct {
	Code        string          `json:"code"`
	Msg         string          `json:"msg"`
	RequestTime int64           `json:"requestTime"`
	Data        json.RawMessage `json:"data"` // Використовуємо RawMessage для гнучкості
}

func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	log.Println("Bitget: Початок отримання даних про ставки фінансування...")

	// 1. Отримати всі тікери для USDT-FUTURES
	tickersURL := fmt.Sprintf("%s%s?productType=%s", bitgetAPIEndpointBase, tickersPathV2, productTypeUSDT)
	log.Printf("Bitget: Запит до API для тікерів: %s", tickersURL)

	client := http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(tickersURL)
	if err != nil {
		log.Printf("Bitget: Помилка HTTP GET запиту для тікерів (%s): %v", tickersURL, err)
		return nil, fmt.Errorf("HTTP GET запит до Bitget (тікери): %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		log.Printf("Bitget: Помилка читання тіла відповіді для тікерів: %v", errRead)
		return nil, fmt.Errorf("читання тіла відповіді Bitget (тікери): %w", errRead)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Bitget: Помилка статусу %d від %s (тікери). Тіло: %s", resp.StatusCode, tickersURL, string(bodyBytes))
		return nil, fmt.Errorf("статус %d від Bitget API (тікери) %s", resp.StatusCode, tickersURL)
	}

	var tickersResponse struct { // Bitget V2 /tickers повертає об'єкт з полем data, що є масивом
		Code string            `json:"code"`
		Msg  string            `json:"msg"`
		Data []BitgetTickerV2 `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &tickersResponse); err != nil {
		log.Printf("Bitget: Помилка декодування JSON відповіді для тікерів: %v. Сира відповідь: %s", err, string(bodyBytes[:min(1000, len(bodyBytes))]))
		return nil, fmt.Errorf("розбір JSON відповіді від Bitget (тікери): %w", err)
	}

	if tickersResponse.Code != "00000" {
		log.Printf("Bitget: API (тікери) повернуло помилку: Code=%s, Msg=%s", tickersResponse.Code, tickersResponse.Msg)
		return nil, fmt.Errorf("API Bitget (тікери) повернуло помилку: %s (код %s)", tickersResponse.Msg, tickersResponse.Code)
	}

	allTickers := tickersResponse.Data
	log.Printf("Bitget: Отримано %d тікерів.", len(allTickers))

	var usdtSwapTickers []BitgetTickerV2
	for _, ticker := range allTickers {
		if ticker.QuoteVolume != "" && strings.HasSuffix(ticker.Symbol, "USDT") {
			usdtSwapTickers = append(usdtSwapTickers, ticker)
		}
	}
	log.Printf("Bitget: Відфільтровано %d USDT тікерів з обсягом.", len(usdtSwapTickers))

	sort.SliceStable(usdtSwapTickers, func(i, j int) bool {
		volI, _ := strconv.ParseFloat(usdtSwapTickers[i].QuoteVolume, 64)
		volJ, _ := strconv.ParseFloat(usdtSwapTickers[j].QuoteVolume, 64)
		return volI > volJ
	})

	var tickersToProcess []BitgetTickerV2
	if len(usdtSwapTickers) > topNByVolume {
		log.Printf("Bitget: Обмежуємо обробку до топ-%d з %d знайдених USDT тікерів за обсягом.", topNByVolume, len(usdtSwapTickers))
		tickersToProcess = usdtSwapTickers[:topNByVolume]
	} else {
		log.Printf("Bitget: Знайдено %d USDT тікерів для обробки.", len(usdtSwapTickers))
		tickersToProcess = usdtSwapTickers
	}

	if len(tickersToProcess) == 0 {
		log.Println("Bitget: Не знайдено USDT тікерів для запиту деталей ставок фінансування після фільтрації.")
		return []exchanges.UnifiedFundingRateInfo{}, nil
	}

	var fundingData []exchanges.UnifiedFundingRateInfo
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, maxConcurrentRequests) // Семфор для запитів NextFundingTime

	var errorCountNFT int32
	var processedCount int32

	for _, tickerItem := range tickersToProcess {
		// Перевіряємо основні дані з тікера
		if tickerItem.MarkPrice == "" || tickerItem.FundingRate == "" {
			// log.Printf("Bitget_DEBUG: Пропуск символу %s через порожні MarkPrice або FundingRate з тікера", tickerItem.Symbol)
			continue
		}
		
		wg.Add(1)
		sem <- struct{}{}

		go func(item BitgetTickerV2) {
			defer wg.Done()
			defer func() { <-sem }()

			// Отримуємо NextFundingTime окремим запитом
			var fundingTimeRespData struct { // Bitget V2 /funding-time повертає об'єкт з полем data, що є ОБ'ЄКТОМ
				Code string                         `json:"code"`
				Msg  string                         `json:"msg"`
				Data *BitgetFundingTimeResponseData `json:"data"` // Вказівник, бо може бути null
			}
			
			fundingTimeURL := fmt.Sprintf("%s%s?productType=%s&symbol=%s", bitgetAPIEndpointBase, fundingTimePathV2, productTypeUSDT, item.Symbol)
			
			ftResp, ftErr := client.Get(fundingTimeURL) // Використовуємо той самий клієнт
			if ftErr != nil {
				mu.Lock()
				if errorCountNFT < maxErrorLogs {log.Printf("Bitget: Помилка HTTP GET funding_time для %s: %v", item.Symbol, ftErr)}
				errorCountNFT++
				mu.Unlock()
				return
			}
			defer ftResp.Body.Close()

			ftBodyBytes, ftErrRead := io.ReadAll(ftResp.Body)
			if ftErrRead != nil {
				mu.Lock()
				if errorCountNFT < maxErrorLogs {log.Printf("Bitget: Помилка читання тіла funding_time для %s: %v", item.Symbol, ftErrRead)}
				errorCountNFT++
				mu.Unlock()
				return
			}
			if ftResp.StatusCode != http.StatusOK {
				mu.Lock()
				if errorCountNFT < maxErrorLogs {log.Printf("Bitget: Помилка статусу %d при отриманні funding_time для %s. Тіло: %s", ftResp.StatusCode, item.Symbol, string(ftBodyBytes))}
				errorCountNFT++
				mu.Unlock()
				return
			}
			if err := json.Unmarshal(ftBodyBytes, &fundingTimeRespData); err != nil {
				mu.Lock()
				if errorCountNFT < maxErrorLogs {log.Printf("Bitget: Помилка декодування JSON funding_time для %s: %v. Тіло: %s", item.Symbol, err, string(ftBodyBytes))}
				errorCountNFT++
				mu.Unlock()
				return
			}

			if fundingTimeRespData.Code != "00000" || fundingTimeRespData.Data == nil || fundingTimeRespData.Data.NextFundingTime == "" {
				mu.Lock()
				if errorCountNFT < maxErrorLogs {log.Printf("Bitget: API funding_time для %s повернуло неуспіх або порожні дані. Code: %s, Data: %+v", item.Symbol, fundingTimeRespData.Code, fundingTimeRespData.Data)}
				errorCountNFT++
				mu.Unlock()
				return
			}
			
			markPrice, _ := strconv.ParseFloat(item.MarkPrice, 64) // Вже перевірили, що не порожнє
			fundingRateRaw, _ := strconv.ParseFloat(item.FundingRate, 64) // Вже перевірили
			fundingRatePercent := fundingRateRaw * 100

			nextFundingTimeMs, _ := strconv.ParseInt(fundingTimeRespData.Data.NextFundingTime, 10, 64)
			nextFundingTime := time.Unix(0, nextFundingTimeMs*int64(time.Millisecond)).UTC()
			
			symbolClean := item.Symbol 

			mu.Lock()
			fundingData = append(fundingData, exchanges.UnifiedFundingRateInfo{
				Exchange:        "Bitget",
				Symbol:          symbolClean,
				MarkPrice:       markPrice,
				LastFundingRate: fundingRatePercent,
				NextFundingTime: nextFundingTime,
			})
			processedCount++
			mu.Unlock()
		}(tickerItem)
	}
	wg.Wait()

	log.Printf("Bitget: Успішно оброблено та зібрано дані фінансування для %d USDT пар (з %d відфільтрованих за обсягом). Помилок NextFundingTime: %d.",
		processedCount, len(tickersToProcess), errorCountNFT)
	return fundingData, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
