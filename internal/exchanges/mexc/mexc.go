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
	fundingRatePath     = "/funding_rate/"
	fairPricePath       = "/fair_price/"
	maxConcurrentRequests = 5 // ЗМЕНШЕНО для обережності з лімітами
	maxErrorLogs        = 5 // Максимальна кількість помилок кожного типу для логування
)

type MEXCContractDetail struct {
	Symbol          string  `json:"symbol"`
	DisplayName     string  `json:"displayName"`
	State           int     `json:"state"`
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
}

type MEXCFundingRateInfo struct {
	Symbol          string  `json:"symbol"`
	FundingRate     float64 `json:"fundingRate"`
	NextFundingTime int64   `json:"nextFundingTime"`
}

type MEXCFairPriceInfo struct {
	Symbol    string  `json:"symbol"`
	FairPrice float64 `json:"fairPrice"`
}

type MEXCAPIResponseWrapper struct {
	Success bool            `json:"success,omitempty"`
	Code    int             `json:"code,omitempty"`
	Msg     string          `json:"msg,omitempty"`
	Data    json.RawMessage `json:"data"`
}

func fetchMEXCSingleObjectData(url string, target interface{}) error {
	client := http.Client{Timeout: 20 * time.Second} // Трохи збільшимо таймаут
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET до %s: %w", url, err)
	}
	defer resp.Body.Close()

	bodyBytes, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		return fmt.Errorf("читання тіла відповіді від %s: %w", url, errRead)
	}

	if resp.StatusCode != http.StatusOK {
		// Не логуємо тіло тут, бо воно може бути великим і неінформативним для простої помилки статусу
		return fmt.Errorf("статус %d від %s", resp.StatusCode, url)
	}

	var wrapper MEXCAPIResponseWrapper
	errUnmarshalWrapper := json.Unmarshal(bodyBytes, &wrapper)

	if errUnmarshalWrapper == nil && (wrapper.Code == 200 || wrapper.Code == 0 || wrapper.Success) {
		if len(wrapper.Data) > 0 && string(wrapper.Data) != "null" {
			if err := json.Unmarshal(wrapper.Data, target); err != nil {
				return fmt.Errorf("декодування поля 'data' (%s) від %s: %w. Raw 'data': %s", string(wrapper.Data), url, err, string(wrapper.Data))
			}
			return nil
		}
		if errDirect := json.Unmarshal(bodyBytes, target); errDirect == nil {
			return nil
		}
		return nil
	}

	if errDirect := json.Unmarshal(bodyBytes, target); errDirect != nil {
		// log.Printf("MEXC: Помилка прямого декодування відповіді від %s: %v. Сира відповідь: %s", url, errDirect, string(bodyBytes)) // Може бути занадто багато логів
		return fmt.Errorf("декодування прямої відповіді від %s: %w (також не вдалося розпарсити як обгортку: %v)", url, errDirect, errUnmarshalWrapper)
	}
	return nil
}

func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	log.Println("MEXC: Початок отримання даних про ставки фінансування...")

	var allContracts []MEXCContractDetail
	contractsURL := mexcAPIEndpointBase + allContractsPath

	client := http.Client{Timeout: 20 * time.Second} // Збільшено таймаут
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
	
	var detailWrapper struct {
		Code    int                  `json:"code"`
		Data    []MEXCContractDetail `json:"data"`
		Msg     string               `json:"msg,omitempty"`
	}

	bodyBytes, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		log.Printf("MEXC: Помилка читання тіла відповіді для /detail: %v", errRead)
		return nil, fmt.Errorf("читання тіла відповіді MEXC /detail: %w", errRead)
	}

	if err := json.Unmarshal(bodyBytes, &detailWrapper); err != nil {
		log.Printf("MEXC: Помилка декодування обгортки списку інструментів: %v. Сира відповідь (перші 500 байт): %s", err, string(bodyBytes[:min(500,len(bodyBytes))]))
		return nil, fmt.Errorf("декодування обгортки інструментів MEXC: %w", err)
	}

	if detailWrapper.Code != 0 && detailWrapper.Code != 200 { // MEXC використовує code 0 або 200 для успіху
		log.Printf("MEXC: API /detail повернуло помилку: code %d, msg: %s", detailWrapper.Code, detailWrapper.Msg)
		return nil, fmt.Errorf("API MEXC /detail повернуло помилку: %s (код %d)", detailWrapper.Msg, detailWrapper.Code)
	}
	allContracts = detailWrapper.Data
	
	log.Printf("MEXC: Отримано %d контрактів.", len(allContracts))

	var usdtSwapSymbols []string
	for _, contract := range allContracts {
		if contract.State == 0 && contract.SettleCoin == "USDT" && strings.HasSuffix(contract.Symbol, "_USDT") {
			usdtSwapSymbols = append(usdtSwapSymbols, contract.Symbol)
		}
	}
	log.Printf("MEXC: Знайдено %d активних USDT_SWAP контрактів.", len(usdtSwapSymbols))
	if len(usdtSwapSymbols) == 0 {
		log.Println("MEXC: Не знайдено активних USDT_SWAP контрактів для обробки.")
		return []exchanges.UnifiedFundingRateInfo{}, nil
	}

	var fundingData []exchanges.UnifiedFundingRateInfo
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, maxConcurrentRequests)

	var fairPriceErrorCount int32
	var fundingRateErrorCount int32
	var fairPriceEmptyCount int32
	var fundingRateEmptyCount int32


	for _, symbol := range usdtSwapSymbols {
		wg.Add(1)
		sem <- struct{}{}

		go func(s string) {
			defer wg.Done()
			defer func() { <-sem }()

			var fairPriceInfo MEXCFairPriceInfo
			fairPriceURL := mexcAPIEndpointBase + fairPricePath + s
			if err := fetchMEXCSingleObjectData(fairPriceURL, &fairPriceInfo); err != nil {
				mu.Lock()
				if fairPriceErrorCount < maxErrorLogs {
					log.Printf("MEXC: Помилка отримання fair_price для %s: %v", s, err)
					fairPriceErrorCount++
				}
				mu.Unlock()
				return
			}
			if fairPriceInfo.Symbol == "" { 
				mu.Lock()
				if fairPriceEmptyCount < maxErrorLogs {
					// log.Printf("MEXC: Не знайдено даних fair_price (порожній Symbol) для %s", s)
					fairPriceEmptyCount++
				}
				mu.Unlock()
				return
			}

			var fundingRateInfo MEXCFundingRateInfo
			fundingRateURL := mexcAPIEndpointBase + fundingRatePath + s
			if err := fetchMEXCSingleObjectData(fundingRateURL, &fundingRateInfo); err != nil {
				mu.Lock()
				if fundingRateErrorCount < maxErrorLogs {
					log.Printf("MEXC: Помилка отримання funding_rate для %s: %v", s, err)
					fundingRateErrorCount++
				}
				mu.Unlock()
				return
			}
			 if fundingRateInfo.Symbol == "" {
				mu.Lock()
				if fundingRateEmptyCount < maxErrorLogs {
                	// log.Printf("MEXC: Не знайдено даних funding_rate (порожній Symbol) для %s", s)
					fundingRateEmptyCount++
				}
				mu.Unlock()
                return
            }
			
			fundingRatePercent := fundingRateInfo.FundingRate * 100 
			nextFundingTime := time.Unix(0, fundingRateInfo.NextFundingTime*int64(time.Millisecond)).UTC()
			
			symbolClean := strings.Replace(s, "_", "", 1)

			mu.Lock()
			fundingData = append(fundingData, exchanges.UnifiedFundingRateInfo{
				Exchange:        "MEXC",
				Symbol:          symbolClean,
				MarkPrice:       fairPriceInfo.FairPrice,
				LastFundingRate: fundingRatePercent,
				NextFundingTime: nextFundingTime,
			})
			mu.Unlock()
		}(symbol)
	}
	wg.Wait()

	log.Printf("MEXC: Успішно оброблено та зібрано дані фінансування для %d USDT SWAP пар (з %d спроб). Помилок FairPrice: %d (з них порожніх: %d), Помилок FundingRate: %d (з них порожніх: %d).",
		len(fundingData), len(usdtSwapSymbols), fairPriceErrorCount, fairPriceEmptyCount, fundingRateErrorCount, fundingRateEmptyCount)
	return fundingData, nil
}

// Допоміжна функція min для логування сирої відповіді
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
