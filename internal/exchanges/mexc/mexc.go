package mexc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges"
)

const (
	mexcAPIEndpointBase   = "https://contract.mexc.com/api/v1/contract"
	allContractsPath    = "/detail"
	fundingRatePath     = "/funding_rate/" // Додається {symbol}
	fairPricePath       = "/fair_price/"   // Додається {symbol}
	maxConcurrentRequests = 5                // Обмеження одночасних запитів до API MEXC
	maxErrorLogs        = 5                // Максимальна кількість помилок кожного типу для логування
)

// --- Структури для відповіді API MEXC ---

type MEXCContractDetail struct {
	Symbol          string  `json:"symbol"`
	DisplayName     string  `json:"displayName"`
	State           int     `json:"state"` // 0:SHOWING, 1:HIDE, 2:SUSPENDED
	SettleCoin      string  `json:"settleCoin"`
	BaseCoin        string  `json:"baseCoin"`
	QuoteCoin       string  `json:"quoteCoin"`
	ContractSize    float64 `json:"contractSize"`
	MinLeverage     int     `json:"minLeverage"`
	MaxLeverage     int     `json:"maxLeverage"`
	PriceScale      int     `json:"priceScale"`
	VolScale        int     `json:"volScale"`
	AmountScale     int     `json:"amountScale"`
	FundingInterval int     `json:"fundingInterval"`
	// Додамо поле для обсягу, якщо воно є; документація не дуже чітка,
	// але спробуємо знайти щось схоже на volValue24h або quoteVolume24h
	// Поки що залишимо без явного поля обсягу, будемо брати всі активні USDT пари.
	// Якщо відповідь буде занадто довгою, повернемося до цього.
}

type MEXCFundingRateInfo struct {
	Success         bool    `json:"success"` // MEXC часто використовує це поле
	Code            int     `json:"code"`
	Data            *MEXCFundingRateData `json:"data,omitempty"` // Використовуємо вказівник, щоб перевірити на nil
}
type MEXCFundingRateData struct {
	Symbol          string  `json:"symbol"`
	FundingRate     float64 `json:"fundingRate"`   // Ставка як десяткове число (0.0001 для 0.01%)
	NextFundingTime int64   `json:"nextFundingTime"` // Час наступної виплати (UTC ms)
}

type MEXCFairPriceInfo struct {
	Success   bool    `json:"success"`
	Code      int     `json:"code"`
	Data      *MEXCFairPriceData `json:"data,omitempty"`
}
type MEXCFairPriceData struct {
	Symbol    string  `json:"symbol"`
	FairPrice float64 `json:"fairPrice"` // Ціна маркування
}

// MEXCAllContractsResponse для /api/v1/contract/detail
type MEXCAllContractsResponse struct {
	Success bool                 `json:"success"`
	Code    int                  `json:"code"`
	Data    []MEXCContractDetail `json:"data"`
}


// GetFundingRates отримує ставки фінансування для USDT-M контрактів з MEXC
func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	log.Println("MEXC: Початок отримання даних про ставки фінансування...")

	var allContractsResponse MEXCAllContractsResponse
	contractsURL := mexcAPIEndpointBase + allContractsPath
	
	client := http.Client{Timeout: 20 * time.Second} // Збільшено таймаут для /detail
	resp, err := client.Get(contractsURL)
	if err != nil {
		log.Printf("MEXC: Помилка HTTP GET запиту для отримання списку інструментів (%s): %v", contractsURL, err)
		return nil, fmt.Errorf("отримання інструментів MEXC: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("MEXC: Помилка статусу при отриманні списку інструментів (%s): %s. Тіло: %s", contractsURL, resp.Status, string(bodyBytes))
		return nil, fmt.Errorf("статус %d від %s", resp.StatusCode, contractsURL)
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&allContractsResponse); err != nil {
		bodyBytesForLog, _ := io.ReadAll(io.NopCloser(resp.Body)) // Спробуємо прочитати для логу, якщо можливо
		log.Printf("MEXC: Помилка декодування відповіді від /detail: %v. Сира відповідь (якщо вдалося прочитати): %s", err, string(bodyBytesForLog))
		return nil, fmt.Errorf("декодування відповіді /detail MEXC: %w", err)
	}

	if !allContractsResponse.Success || (allContractsResponse.Code != 0 && allContractsResponse.Code != 200) {
		log.Printf("MEXC: API /detail повернуло неуспішний результат: success=%t, code=%d", allContractsResponse.Success, allContractsResponse.Code)
		return nil, fmt.Errorf("API MEXC /detail повернуло неуспішний результат: success=%t, code=%d", allContractsResponse.Success, allContractsResponse.Code)
	}
	
	allContracts := allContractsResponse.Data
	log.Printf("MEXC: Отримано %d контрактів з /detail.", len(allContracts))

	var usdtSwapSymbols []string
	for _, contract := range allContracts {
		if contract.State == 0 && contract.SettleCoin == "USDT" && strings.HasSuffix(contract.Symbol, "_USDT") {
			usdtSwapSymbols = append(usdtSwapSymbols, contract.Symbol)
		}
	}
	log.Printf("MEXC: Знайдено %d активних USDT_SWAP контрактів для обробки.", len(usdtSwapSymbols))
	if len(usdtSwapSymbols) == 0 {
		return []exchanges.UnifiedFundingRateInfo{}, nil
	}
	
	// Обмежимо кількість символів для обробки, щоб прискорити
	processingLimit := 75 // Беремо перші N символів зі списку (список не відсортований за обсягом)
	if len(usdtSwapSymbols) > processingLimit {
		log.Printf("MEXC: Обмежуємо обробку до перших %d з %d знайдених USDT_SWAP контрактів.", processingLimit, len(usdtSwapSymbols))
		usdtSwapSymbols = usdtSwapSymbols[:processingLimit]
	}


	var fundingData []exchanges.UnifiedFundingRateInfo
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, maxConcurrentRequests)

	var fairPriceErrorCount int32
	var fundingRateErrorCount int32
	var processedCount int32


	for _, symbol := range usdtSwapSymbols {
		wg.Add(1)
		sem <- struct{}{}

		go func(s string) {
			defer wg.Done()
			defer func() { <-sem }()

			// 1. Отримати Fair Price (Mark Price)
			var fairPriceResponse MEXCFairPriceInfo
			fairPriceURL := mexcAPIEndpointBase + fairPricePath + s
			
			httpClient := http.Client{Timeout: 10 * time.Second}
			fpResp, fpErr := httpClient.Get(fairPriceURL)
			if fpErr != nil {
				mu.Lock()
				if fairPriceErrorCount < maxErrorLogs {
					log.Printf("MEXC: Помилка HTTP GET fair_price для %s: %v", s, fpErr)
				}
				fairPriceErrorCount++
				mu.Unlock()
				return
			}
			defer fpResp.Body.Close()
			if fpResp.StatusCode != http.StatusOK {
				mu.Lock()
				if fairPriceErrorCount < maxErrorLogs {
					log.Printf("MEXC: Помилка статусу %d при отриманні fair_price для %s", fpResp.StatusCode, s)
				}
				fairPriceErrorCount++
				mu.Unlock()
				return
			}
			if err := json.NewDecoder(fpResp.Body).Decode(&fairPriceResponse); err != nil {
				mu.Lock()
				if fairPriceErrorCount < maxErrorLogs {
					log.Printf("MEXC: Помилка декодування fair_price для %s: %v", s, err)
				}
				fairPriceErrorCount++
				mu.Unlock()
				return
			}
			if !fairPriceResponse.Success || (fairPriceResponse.Code !=0 && fairPriceResponse.Code !=200) || fairPriceResponse.Data == nil {
				mu.Lock()
				if fairPriceErrorCount < maxErrorLogs {
					log.Printf("MEXC: API fair_price для %s повернуло неуспіх або порожні дані: success=%t, code=%d", s, fairPriceResponse.Success, fairPriceResponse.Code)
				}
				fairPriceErrorCount++
				mu.Unlock()
				return
			}


			// 2. Отримати Funding Rate
			var fundingRateResponse MEXCFundingRateInfo
			fundingRateURL := mexcAPIEndpointBase + fundingRatePath + s
			
			frResp, frErr := httpClient.Get(fundingRateURL)
			if frErr != nil {
				mu.Lock()
				if fundingRateErrorCount < maxErrorLogs {
					log.Printf("MEXC: Помилка HTTP GET funding_rate для %s: %v", s, frErr)
				}
				fundingRateErrorCount++
				mu.Unlock()
				return
			}
			defer frResp.Body.Close()
			if frResp.StatusCode != http.StatusOK {
				mu.Lock()
				if fundingRateErrorCount < maxErrorLogs {
					log.Printf("MEXC: Помилка статусу %d при отриманні funding_rate для %s", frResp.StatusCode, s)
				}
				fundingRateErrorCount++
				mu.Unlock()
				return
			}
			if err := json.NewDecoder(frResp.Body).Decode(&fundingRateResponse); err != nil {
				mu.Lock()
				if fundingRateErrorCount < maxErrorLogs {
					log.Printf("MEXC: Помилка декодування funding_rate для %s: %v", s, err)
				}
				fundingRateErrorCount++
				mu.Unlock()
				return
			}
			if !fundingRateResponse.Success || (fundingRateResponse.Code !=0 && fundingRateResponse.Code !=200) || fundingRateResponse.Data == nil {
				mu.Lock()
				if fundingRateErrorCount < maxErrorLogs {
                	log.Printf("MEXC: API funding_rate для %s повернуло неуспіх або порожні дані: success=%t, code=%d", s, fundingRateResponse.Success, fundingRateResponse.Code)
				}
				fundingRateErrorCount++
				mu.Unlock()
                return
            }
			
			fundingRateDataAPI := fundingRateResponse.Data
			fairPriceDataAPI := fairPriceResponse.Data

			// Перевіряємо, чи дані не nil
			if fundingRateDataAPI == nil || fairPriceDataAPI == nil {
				// log.Printf("MEXC: Одне з полів data є nil для %s", s)
				return
			}

			fundingRatePercent := fundingRateDataAPI.FundingRate * 100 
			nextFundingTime := time.Unix(0, fundingRateDataAPI.NextFundingTime*int64(time.Millisecond)).UTC()
			
			symbolClean := strings.Replace(s, "_", "", 1)

			mu.Lock()
			fundingData = append(fundingData, exchanges.UnifiedFundingRateInfo{
				Exchange:        "MEXC",
				Symbol:          symbolClean,
				MarkPrice:       fairPriceDataAPI.FairPrice,
				LastFundingRate: fundingRatePercent,
				NextFundingTime: nextFundingTime,
			})
			processedCount++
			mu.Unlock()
		}(symbol)
	}
	wg.Wait()

	log.Printf("MEXC: Успішно оброблено та зібрано дані фінансування для %d з %d USDT SWAP пар. Помилок FairPrice: %d, Помилок FundingRate: %d.",
		processedCount, len(usdtSwapSymbols), fairPriceErrorCount, fundingRateErrorCount)
	return fundingData, nil
}

// min функція не використовується, якщо логуємо лише перші 500 байт сирої відповіді
// func min(a, b int) int {
// 	if a < b {
// 		return a
// 	}
// 	return b
// }
