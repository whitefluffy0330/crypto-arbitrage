package mexc

import (
	"encoding/json"
	"fmt"
	"io" // Потрібен для io.ReadAll
	"log"
	"net/http"
	// "strconv" // Більше не потрібен, якщо API повертає числа
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

// MEXCAPIResponseWrapper - загальна структура обгортки для відповіді MEXC
type MEXCAPIResponseWrapper struct {
	Success bool            `json:"success,omitempty"`
	Code    int             `json:"code,omitempty"` // 0 або 200 зазвичай успіх
	Msg     string          `json:"msg,omitempty"`
	Data    json.RawMessage `json:"data"` // Поле 'data' може містити об'єкт або масив
}

// fetchMEXCAndUnmarshalData робить GET-запит та розпаковує поле "data" у target
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
		// Якщо не вдалося розпарсити як обгортку, можливо, відповідь - це безпосередньо дані (наприклад, масив для /detail)
		// log.Printf("MEXC: Не вдалося розпарсити як обгортку для %s, спроба прямого парсингу: %v. Тіло: %s", url, err, string(bodyBytes))
		if errDirect := json.Unmarshal(bodyBytes, target); errDirect != nil {
			return fmt.Errorf("декодування прямої відповіді від %s: %w (після невдалої спроби обгортки: %v)", url, errDirect, err)
		}
		// log.Printf("MEXC: Успішний прямий парсинг для %s", url)
		return nil
	}

	// Перевіряємо код відповіді з обгортки
	// MEXC може повертати success:true або code:200 (або code:0) для успіху
	if !wrapper.Success && wrapper.Code != 200 && wrapper.Code != 0 {
		return fmt.Errorf("API MEXC (%s) повернуло помилку: %s (код %d)", url, wrapper.Msg, wrapper.Code)
	}

	if len(wrapper.Data) == 0 || string(wrapper.Data) == "null" {
		// log.Printf("MEXC: Поле 'data' порожнє або null у відповіді від %s", url)
		// Це може бути нормально для деяких запитів, що не повертають дані,
		// але для /detail, /funding_rate, /fair_price ми очікуємо дані.
		// Якщо target - це зріз, він залишиться порожнім, що коректно.
		// Якщо target - структура, і дані порожні, то поля залишаться нульовими.
		return nil // Не вважаємо це помилкою тут, нехай викликаючий код перевіряє порожнечу target
	}

	// Розпаковуємо поле "data" у цільову структуру/зріз
	if err := json.Unmarshal(wrapper.Data, target); err != nil {
		return fmt.Errorf("декодування поля 'data' (%s) від %s: %w. Raw 'data': %s", wrapper.Data, url, err, string(wrapper.Data))
	}
	return nil
}

func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	log.Println("MEXC: Початок отримання даних про ставки фінансування...")

	var allContractsData struct { // Очікуємо, що /detail може бути загорнутий в "data"
		Data []MEXCContractDetail `json:"data"`
	}
	// Альтернативно, якщо /detail повертає масив напряму, то:
	var allContractsDirect []MEXCContractDetail

	contractsURL := mexcAPIEndpointBase + allContractsPath
	
	// Спроба розпарсити відповідь /detail
	// Спочатку спробуємо розпарсити як об'єкт з полем "data", що містить масив
	// Якщо не вийде, спробуємо розпарсити як простий масив.
	// На основі документації, /detail має повертати { "success": true, "code": 0, "data": [...] }

	if err := fetchMEXCAndUnmarshalData(contractsURL, &allContractsDirect); err != nil {
		log.Printf("MEXC: Помилка отримання/декодування списку інструментів: %v", err)
		return nil, fmt.Errorf("отримання/декодування інструментів MEXC: %w", err)
	}
	// Після fetchMEXCAndUnmarshalData, allContractsDirect має бути заповнений, якщо відповідь була масивом
	// або якщо вона була об'єктом {"data": [...]}, а target був *[]MEXCContractDetail
	
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
			// Для /fair_price/{symbol} відповідь зазвичай { "success": true, "code": 0, "data": { "symbol": "BTC_USDT", "fairPrice": 65000.0 } }
			// або просто { "symbol": "BTC_USDT", "fairPrice": 65000.0 }
			if err := fetchMEXCAndUnmarshalData(fairPriceURL, &fairPriceInfo); err != nil {
				// log.Printf("MEXC: Помилка отримання fair_price для %s: %v", s, err)
				return
			}
			if fairPriceInfo.Symbol == "" {
				return
			}

			var fundingRateInfo MEXCFundingRateInfo
			fundingRateURL := mexcAPIEndpointBase + fundingRatePath + s
			// Аналогічно для /funding_rate/{symbol}
			if err := fetchMEXCAndUnmarshalData(fundingRateURL, &fundingRateInfo); err != nil {
				// log.Printf("MEXC: Помилка отримання funding_rate для %s: %v", s, err)
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
