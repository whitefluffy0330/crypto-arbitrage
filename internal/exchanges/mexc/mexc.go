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
	maxConcurrentRequests = 10
)

type MEXCContractDetail struct {
	Symbol          string  `json:"symbol"`
	DisplayName     string  `json:"displayName"`
	State           int     `json:"state"` // ЗМІНЕНО ТИП НА INT
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
	client := http.Client{Timeout: 15 * time.Second}
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
		log.Printf("MEXC: Помилка статусу %d від %s. Тіло: %s", resp.StatusCode, url, string(bodyBytes))
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
		log.Printf("MEXC: Помилка прямого декодування відповіді від %s: %v. Сира відповідь: %s", url, errDirect, string(bodyBytes))
		return fmt.Errorf("декодування прямої відповіді від %s: %w (також не вдалося розпарсити як обгортку: %v)", url, errDirect, errUnmarshalWrapper)
	}
	return nil
}

func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	log.Println("MEXC: Початок отримання даних про ставки фінансування...")

	var allContracts []MEXCContractDetail
	contractsURL := mexcAPIEndpointBase + allContractsPath

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(contractsURL)
	if err != nil {
		log.Printf("MEXC: Помилка HTTP GET запиту для отримання списку інструментів (%s): %v", contractsURL, err)
		return nil, fmt.Errorf("отримання інструментів MEXC: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("MEXC: Помилка статусу при отриманні списку інструментів (%s): %s", contractsURL, resp.Status)
		return nil, fmt.Errorf("статус %d від %s", resp.StatusCode, contractsURL)
	}
	
	var detailWrapper struct {
		// Success bool `json:"success"` // Поле success може бути відсутнім, орієнтуємося на code
		Code    int                  `json:"code"`    // Очікуємо 0 або 200 для успіху
		Data    []MEXCContractDetail `json:"data"`
		Msg     string               `json:"msg,omitempty"` // Додамо поле Msg для діагностики
	}

	bodyBytes, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		log.Printf("MEXC: Помилка читання тіла відповіді для /detail: %v", errRead)
		return nil, fmt.Errorf("читання тіла відповіді MEXC /detail: %w", errRead)
	}

	if err := json.Unmarshal(bodyBytes, &detailWrapper); err != nil {
		log.Printf("MEXC: Помилка декодування обгортки списку інструментів: %v. Сира відповідь: %s", err, string(bodyBytes))
		return nil, fmt.Errorf("декодування обгортки інструментів MEXC: %w", err)
	}

	// MEXC повертає code: 0 для успіху в цьому ендпоінті (згідно з їхньою документацією)
	if detailWrapper.Code != 0 {
		log.Printf("MEXC: API /detail повернуло помилку: code %d, msg: %s", detailWrapper.Code, detailWrapper.Msg)
		return nil, fmt.Errorf("API MEXC /detail повернуло помилку: %s (код %d)", detailWrapper.Msg, detailWrapper.Code)
	}
	allContracts = detailWrapper.Data
	
	log.Printf("MEXC: Отримано %d контрактів.", len(allContracts))

	var usdtSwapSymbols []string
	for _, contract := range allContracts {
		// MEXC API: state (0:SHOWING, 1:HIDE, 2:SUSPENDED)
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

	for _, symbol := range usdtSwapSymbols {
		wg.Add(1)
		sem <- struct{}{}

		go func(s string) {
			defer wg.Done()
			defer func() { <-sem }()

			var fairPriceInfo MEXCFairPriceInfo
			fairPriceURL := mexcAPIEndpointBase + fairPricePath + s
			if err := fetchMEXCSingleObjectData(fairPriceURL, &fairPriceInfo); err != nil {
				return
			}
			if fairPriceInfo.Symbol == "" { 
				return
			}

			var fundingRateInfo MEXCFundingRateInfo
			fundingRateURL := mexcAPIEndpointBase + fundingRatePath + s
			if err := fetchMEXCSingleObjectData(fundingRateURL, &fundingRateInfo); err != nil {
				return
			}
			 if fundingRateInfo.Symbol == "" {
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

	log.Printf("MEXC: Успішно оброблено та зібрано дані фінансування для %d USDT SWAP пар.", len(fundingData))
	return fundingData, nil
}
