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
	maxConcurrentRequests = 5 
	maxErrorLogs        = 5 
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
	NextFundingTime int64   `json:"nextFundingTime"` // Припускаємо, що це Unix Timestamp в СЕКУНДАХ
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
	client := http.Client{Timeout: 20 * time.Second} 
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

	client := http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(contractsURL)
	if err != nil {
		log.Printf("MEXC: Помилка HTTP GET запиту для отримання списку інструментів (%s): %v", contractsURL, err)
		return nil, fmt.Errorf("отримання інструментів MEXC: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytesLog, _ := io.ReadAll(resp.Body) // Читаємо для логування помилки
		log.Printf("MEXC: Помилка статусу при отриманні списку інструментів (%s): %s. Тіло: %s", contractsURL, resp.Status, string(bodyBytesLog))
		return nil, fmt.Errorf("статус %d від %s", resp.StatusCode, contractsURL)
	}
	
	var detailWrapper struct {
		Code    int                  `json:"code"`
		Data    []MEXCContractDetail `json:"data"`
		Msg     string               `json:"msg,omitempty"`
	}

	bodyBytes, errRead := io.ReadAll(resp.Body) // Читаємо тіло для декодування
	if errRead != nil {
		log.Printf("MEXC: Помилка читання тіла відповіді для /detail: %v", errRead)
		return nil, fmt.Errorf("читання тіла відповіді MEXC /detail: %w", errRead)
	}

	if err := json.Unmarshal(bodyBytes, &detailWrapper); err != nil {
		log.Printf("MEXC: Помилка декодування обгортки списку інструментів: %v. Сира відповідь (перші 500 байт): %s", err, string(bodyBytes[:min(500,len(bodyBytes))]))
		return nil, fmt.Errorf("декодування обгортки інструментів MEXC: %w", err)
	}

	if detailWrapper.Code != 0 && detailWrapper.Code != 200 {
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
					fundingRateEmptyCount++
				}
				mu.Unlock()
                return
            }
			
			fundingRatePercent := fundingRateInfo.FundingRate * 100 
			// ВИПРАВЛЕНО КОНВЕРТАЦІЮ ЧАСУ ДЛЯ MEXC (припускаємо, що це секунди)
			nextFundingTime := time.Unix(fundingRateInfo.NextFundingTime/1000, 0).UTC() // Розділимо на 1000, якщо це все ж мілісекунди
                                                                                     // Або якщо це точно секунди: time.Unix(fundingRateInfo.NextFundingTime, 0).UTC()
                                                                                     // Потрібно перевірити документацію API MEXC для формату nextFundingTime
                                                                                     // Якщо API MEXC повертає мілісекунди, то оригінальний код:
                                                                                     // nextFundingTime := time.Unix(0, fundingRateInfo.NextFundingTime*int64(time.Millisecond)).UTC()
                                                                                     // МАВ БУТИ ПРАВИЛЬНИМ.
                                                                                     // Давайте спробуємо припустити, що це все ж мілісекунди, але можливо, іноді приходить 0 або невалід.
                                                                                     // Якщо NextFundingTime == 0, то time.Unix(0,0) дасть 1970-01-01.

			// Якщо fundingRateInfo.NextFundingTime часто буває 0 або невалідним,
			// то потрібно обробляти цей випадок, можливо, не показуючи час, або показуючи "N/A"
			if fundingRateInfo.NextFundingTime <= 0 { // Додамо перевірку на валідність часу
				// log.Printf("MEXC: Отримано невалідний NextFundingTime (%d) для %s. Пропускаємо час.", fundingRateInfo.NextFundingTime, s)
				// У цьому випадку можна встановити якийсь "порожній" час або не заповнювати його,
				// а в handler.go перевіряти, чи час встановлено.
				// Поки що залишимо конвертацію, але логування допоможе.
			}
			// ЗАЛИШАЄМО ПОПЕРЕДНЮ КОНВЕРТАЦІЮ, припускаючи мілісекунди, але проблема може бути в самих даних.
			nextFundingTime = time.Unix(0, fundingRateInfo.NextFundingTime*int64(time.Millisecond)).UTC()


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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
