package mexc

import (
	"encoding/json"
	"fmt"
	"io" 
	"log"
	"net/http"
	// "strconv" 
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


func fetchMEXCAndUnmarshalData(url string, target interface{}) error {
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
	if err := json.Unmarshal(bodyBytes, &wrapper); err != nil {
		if errDirect := json.Unmarshal(bodyBytes, target); errDirect != nil {
			return fmt.Errorf("декодування прямої відповіді від %s: %w (після невдалої спроби обгортки: %v)", url, errDirect, err)
		}
		return nil
	}

	if !wrapper.Success && wrapper.Code != 200 && wrapper.Code != 0 {
		return fmt.Errorf("API MEXC (%s) повернуло помилку: %s (код %d)", url, wrapper.Msg, wrapper.Code)
	}

	if len(wrapper.Data) == 0 || string(wrapper.Data) == "null" {
		return nil 
	}

	if err := json.Unmarshal(wrapper.Data, target); err != nil {
		return fmt.Errorf("декодування поля 'data' (%s) від %s: %w. Raw 'data': %s", wrapper.Data, url, err, string(wrapper.Data))
	}
	return nil
}

func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	log.Println("MEXC: Початок отримання даних про ставки фінансування...")

	// var allContractsData struct { // ВИДАЛЕНО НЕВИКОРИСТОВУВАНУ ЗМІННУ
	// 	Data []MEXCContractDetail `json:"data"`
	// }
	var allContractsDirect []MEXCContractDetail

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
	
	// Припускаємо, що /detail повертає об'єкт з полем "data", яке містить масив
	var detailResponse struct {
		Data []MEXCContractDetail `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&detailResponse); err != nil {
		// Якщо це не спрацювало, можливо, відповідь - це просто масив
		// Повернемо тіло для повторного читання і спробуємо розпарсити як масив
		// (Це потребує читання тіла в байти спочатку, потім передачі bytes.NewReader)
		// Поки що спрощуємо і очікуємо обгортку або прямий масив, оброблений fetchMEXCAndUnmarshalData
		// АБО для /detail, який має стабільну структуру, можна зробити прямий парсинг.
		// Згідно документації, /detail повертає {"success":true,"code":0,"data":[...]}
		// Тому fetchMEXCAndUnmarshalData має спрацювати, якщо target для нього буде *[]MEXCContractDetail
		// АЛЕ, ми вже розпарсили його вище, тому використовуємо allContractsDirect
		// Переробимо цей блок, щоб він був послідовним
		// Перезавантажимо тіло відповіді, якщо це потрібно, або просто використаємо fetchMEXCAndUnmarshalData
		// Я повернуся до прямого декодування для /detail, як було спочатку, але з перевіркою
		// Якщо документація каже, що /detail ПОВИНЕН мати обгортку, то так і робимо.
		// Якщо у відповіді #277 було `json: cannot unmarshal object into Go value of type []mexc.MEXCContractDetail`
		// це означає, що відповідь НЕ БУЛА масивом, а була об'єктом.
		// Отже, використовуємо fetchMEXCAndUnmarshalData
		if errFetch := fetchMEXCAndUnmarshalData(contractsURL, &allContractsDirect); errFetch != nil {
			log.Printf("MEXC: Помилка отримання/декодування списку інструментів з fetchMEXCAndUnmarshalData: %v", errFetch)
			return nil, fmt.Errorf("отримання/декодування інструментів MEXC з fetchMEXCAndUnmarshalData: %w", errFetch)
		}
	} else {
		allContractsDirect = detailResponse.Data
	}

	log.Printf("MEXC: Отримано %d контрактів.", len(allContractsDirect))

	var usdtSwapSymbols []string
	for _, contract := range allContractsDirect {
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
			if err := fetchMEXCAndUnmarshalData(fairPriceURL, &fairPriceInfo); err != nil {
				return
			}
			if fairPriceInfo.Symbol == "" {
				return
			}

			var fundingRateInfo MEXCFundingRateInfo
			fundingRateURL := mexcAPIEndpointBase + fundingRatePath + s
			if err := fetchMEXCAndUnmarshalData(fundingRateURL, &fundingRateInfo); err != nil {
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
