package mexc

import (
	"bytes"
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

// fetchMEXCAndUnmarshalData для ендпоінтів, які повертають ОДИН об'єкт (можливо, в "data")
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
		// Якщо 'data' порожнє, але код успішний, можливо, відповідь - це сама обгортка (малоймовірно для даних)
		// або просто немає даних. Спробуємо розпарсити тіло як target, якщо target - це *MEXCAPIResponseWrapper
		if errDirect := json.Unmarshal(bodyBytes, target); errDirect == nil {
			// log.Printf("MEXC: Успішний прямий парсинг обгортки (без data) для %s", url)
			return nil
		}
		// log.Printf("MEXC: Поле 'data' порожнє в обгортці для %s, але запит успішний. Target не заповнено з 'data'.", url)
		return nil // Даних немає, але не помилка API
	}

	// Якщо не розпарсилося як обгортка, або код помилки, спробуємо розпарсити напряму
	if errDirect := json.Unmarshal(bodyBytes, target); errDirect != nil {
		log.Printf("MEXC: Помилка прямого декодування відповіді від %s: %v. Сира відповідь: %s", url, errDirect, string(bodyBytes))
		return fmt.Errorf("декодування прямої відповіді від %s: %w (також не вдалося розпарсити як обгортку: %v)", url, errDirect, errUnmarshalWrapper)
	}
	return nil
}

func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	log.Println("MEXC: Початок отримання даних про ставки фінансування...")

	var allContracts []MEXCContractDetail // Очікуємо зріз контрактів
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

	// Ендпоінт /detail для MEXC повертає об'єкт {"success":true, "code":0, "data":[...]}
	// Тому ми маємо розпарсити цю обгортку, а потім поле "data".
	var detailWrapper struct {
		Success bool                 `json:"success"`
		Code    int                  `json:"code"`
		Data    []MEXCContractDetail `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&detailWrapper); err != nil {
		// Якщо не вдалося, спробуємо прочитати тіло ще раз для логування, якщо можливо
		// (resp.Body вже прочитано, тому це не спрацює без збереження bodyBytes)
		log.Printf("MEXC: Помилка декодування обгортки списку інструментів: %v", err)
		// Спробуємо прочитати тіло знову для логування, якщо це можливо (може не спрацювати)
		// Прочитаємо тіло ще раз для логування, якщо можливо
		// Для цього потрібно було б зберегти bodyBytes раніше.
		// Оскільки resp.Body вже прочитано, ми не можемо його прочитати знову тут просто так.
		// Краще буде перевірити логи з попереднього запуску, де була сира відповідь.
		// Але якщо ми дісталися сюди, значить відповідь не була ні [] ни {"data":[]}.
		return nil, fmt.Errorf("декодування обгортки інструментів MEXC: %w", err)
	}

	if !detailWrapper.Success && detailWrapper.Code != 0 && detailWrapper.Code != 200 {
		log.Printf("MEXC: API /detail повернуло помилку: code %d", detailWrapper.Code)
		return nil, fmt.Errorf("API MEXC /detail повернуло помилку: code %d", detailWrapper.Code)
	}
	allContracts = detailWrapper.Data
	
	log.Printf("MEXC: Отримано %d контрактів.", len(allContracts))

	var usdtSwapSymbols []string
	for _, contract := range allContracts {
		if contract.State == "SHOWING" && contract.SettleCoin == "USDT" && strings.HasSuffix(contract.Symbol, "_USDT") {
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
				// log.Printf("MEXC: Помилка отримання fair_price для %s: %v", s, err) // Закоментовано, щоб зменшити спам у логах
				return
			}
			if fairPriceInfo.Symbol == "" { 
				// log.Printf("MEXC: Не знайдено даних fair_price для %s", s)
				return
			}

			var fundingRateInfo MEXCFundingRateInfo
			fundingRateURL := mexcAPIEndpointBase + fundingRatePath + s
			if err := fetchMEXCSingleObjectData(fundingRateURL, &fundingRateInfo); err != nil {
				// log.Printf("MEXC: Помилка отримання funding_rate для %s: %v", s, err)
				return
			}
			if fundingRateInfo.Symbol == "" {
                // log.Printf("MEXC: Не знайдено даних funding_rate для %s", s)
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
