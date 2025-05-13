package mexc

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	// "strconv" // ВИДАЛЕНО НЕПОТРІБНИЙ ІМПОРТ
	"strings"
	"sync"
	"time"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges" // Для UnifiedFundingRateInfo
)

const (
	mexcAPIEndpointBase   = "https://contract.mexc.com/api/v1/contract"
	allContractsPath    = "/detail"
	fundingRatePath     = "/funding_rate/" // Додається {symbol}
	fairPricePath       = "/fair_price/"   // Додається {symbol}
	maxConcurrentRequests = 10               // Обмеження одночасних запитів до API MEXC
)

// --- Структури для відповіді API MEXC ---

type MEXCContractDetail struct {
	Symbol          string  `json:"symbol"`
	DisplayName     string  `json:"displayName"`
	State           string  `json:"state"`
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
	FundingRate     float64 `json:"fundingRate"`   // Очікуємо float64 напряму з JSON
	NextFundingTime int64   `json:"nextFundingTime"` // Час наступної виплати (UTC ms)
}

type MEXCFairPriceInfo struct {
	Symbol    string  `json:"symbol"`
	FairPrice float64 `json:"fairPrice"` // Очікуємо float64 напряму з JSON
}

type MEXCAPIResponseSingle struct {
	Success bool            `json:"success,omitempty"`
	Code    int             `json:"code,omitempty"`
	Msg     string          `json:"msg,omitempty"`
	Data    json.RawMessage `json:"data"`
}

type MEXCAPIResponseList struct {
	Success bool              `json:"success,omitempty"`
	Code    int               `json:"code,omitempty"`
	Msg     string            `json:"msg,omitempty"`
	Data    []json.RawMessage `json:"data"`
}

func fetchMEXCData(url string, target interface{}, targetIsList bool) error {
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET до %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("статус %d від %s", resp.StatusCode, url)
	}

	var rawResponse json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return fmt.Errorf("декодування сирої відповіді від %s: %w", url, err)
	}

	if !targetIsList {
		var genericResponse MEXCAPIResponseSingle
		if errUnmarshalWrapper := json.Unmarshal(rawResponse, &genericResponse); errUnmarshalWrapper == nil {
			if genericResponse.Code != 0 && genericResponse.Code != 200 && genericResponse.Msg != "" {
				return fmt.Errorf("API MEXC (%s) повернуло помилку: %s (код %d)", url, genericResponse.Msg, genericResponse.Code)
			}
			if len(genericResponse.Data) > 0 && string(genericResponse.Data) != "null" {
				if err := json.Unmarshal(genericResponse.Data, target); err != nil {
					return fmt.Errorf("декодування поля 'data' від %s: %w", url, err)
				}
				return nil
			}
		}
		if err := json.Unmarshal(rawResponse, target); err != nil {
			return fmt.Errorf("декодування прямої відповіді від %s: %w (після невдалої спроби обгортки)", url, err)
		}
		return nil
	} else {
		if err := json.Unmarshal(rawResponse, target); err != nil {
			return fmt.Errorf("декодування прямого масиву від %s: %w", url, err)
		}
		return nil
	}
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
	if err := json.NewDecoder(resp.Body).Decode(&allContracts); err != nil {
		log.Printf("MEXC: Помилка декодування списку інструментів: %v", err)
		return nil, fmt.Errorf("декодування інструментів MEXC: %w", err)
	}
	
	log.Printf("MEXC: Отримано %d контрактів.", len(allContracts))

	var usdtSwapSymbols []string
	for _, contract := range allContracts {
		if contract.State == "SHOWING" && contract.SettleCoin == "USDT" && strings.HasSuffix(contract.Symbol, "_USDT") {
			usdtSwapSymbols = append(usdtSwapSymbols, contract.Symbol)
		}
	}
	log.Printf("MEXC: Знайдено %d активних USDT-SWAP контрактів.", len(usdtSwapSymbols))
	if len(usdtSwapSymbols) == 0 {
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
			if err := fetchMEXCData(fairPriceURL, &fairPriceInfo, false); err != nil {
				// log.Printf("MEXC: Помилка отримання fair_price для %s: %v", s, err) // Може бути багато логів
				return
			}
			if fairPriceInfo.Symbol == "" {
				return
			}

			var fundingRateInfo MEXCFundingRateInfo
			fundingRateURL := mexcAPIEndpointBase + fundingRatePath + s
			if err := fetchMEXCData(fundingRateURL, &fundingRateInfo, false); err != nil {
				// log.Printf("MEXC: Помилка отримання funding_rate для %s: %v", s, err) // Може бути багато логів
				return
			}
			 if fundingRateInfo.Symbol == "" {
                return
            }

			// API MEXC для /funding_rate/{symbol} вже повертає fundingRate як float64 (не рядок)
			// і це вже має бути ставка як десяткове число (0.0001 для 0.01%)
			fundingRatePercent := fundingRateInfo.FundingRate * 100 
			nextFundingTime := time.Unix(0, fundingRateInfo.NextFundingTime*int64(time.Millisecond)).UTC()
			
			symbolClean := strings.Replace(s, "_", "", 1)

			mu.Lock()
			fundingData = append(fundingData, exchanges.UnifiedFundingRateInfo{
				Exchange:        "MEXC",
				Symbol:          symbolClean,
				MarkPrice:       fairPriceInfo.FairPrice, // fairPriceInfo.FairPrice вже float64
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
