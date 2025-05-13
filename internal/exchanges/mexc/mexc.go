package mexc

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges" // Для UnifiedFundingRateInfo
)

const (
	mexcAPIEndpointBase   = "https://contract.mexc.com/api/v1/contract"
	allContractsPath      = "/detail"
	fundingRatePath       = "/funding_rate/" // Додається {symbol}
	fairPricePath         = "/fair_price/"   // Додається {symbol}
	maxConcurrentRequests = 10               // Обмеження одночасних запитів до API MEXC
)

// --- Структури для відповіді API MEXC ---

// MEXCContractDetail для /api/v1/contract/detail
type MEXCContractDetail struct {
	Symbol          string  `json:"symbol"` // BTC_USDT
	DisplayName     string  `json:"displayName"`
	State           string  `json:"state"`      // SHOWING, HIDE, SUSPENDED
	SettleCoin      string  `json:"settleCoin"` // USDT, BTC
	BaseCoin        string  `json:"baseCoin"`
	QuoteCoin       string  `json:"quoteCoin"`
	ContractSize    float64 `json:"contractSize"`
	MinLeverage     int     `json:"minLeverage"`
	MaxLeverage     int     `json:"maxLeverage"`
	PriceScale      int     `json:"priceScale"` // Кількість знаків після коми для ціни
	VolScale        int     `json:"volScale"`   // Кількість знаків після коми для обсягу
	AmountScale     int     `json:"amountScale"`
	FundingInterval int     `json:"fundingInterval"` // Інтервал фандингу в годинах
}

// MEXCFundingRateInfo для /api/v1/contract/funding_rate/{symbol}
type MEXCFundingRateInfo struct {
	Symbol          string  `json:"symbol"`
	FundingRate     float64 `json:"fundingRate"`     // Ставка як десяткове число
	NextFundingTime int64   `json:"nextFundingTime"` // Час наступної виплати (UTC ms)
}

// MEXCFairPriceInfo для /api/v1/contract/fair_price/{symbol}
type MEXCFairPriceInfo struct {
	Symbol    string  `json:"symbol"`
	FairPrice float64 `json:"fairPrice"` // Ціна маркування
}

// MEXCAPIResponse структура для загальної відповіді API (якщо вона є)
// MEXC часто повертає дані безпосередньо, або в полі "data" з "code" та "msg"
type MEXCAPIResponseSingle struct {
	Success bool            `json:"success"` // Для деяких ендпоінтів
	Code    int             `json:"code"`    // 0 або 200 зазвичай успіх
	Data    json.RawMessage `json:"data"`
}

type MEXCAPIResponseList struct {
	Success bool              `json:"success"`
	Code    int               `json:"code"`
	Data    []json.RawMessage `json:"data"`
}

// fetchMEXCData робить GET-запит та розпаковує відповідь у надану структуру
// targetIsList вказує, чи очікується список у полі "data"
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

	// Декодуємо в тимчасову структуру, щоб перевірити success/code
	var rawResponse json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return fmt.Errorf("декодування сирої відповіді від %s: %w", url, err)
	}

	// Спробуємо розпарсити як MEXCAPIResponseSingle (для funding_rate, fair_price)
	if !targetIsList {
		var genericResponse MEXCAPIResponseSingle
		if err := json.Unmarshal(rawResponse, &genericResponse); err == nil {
			// Деякі ендпоінти MEXC повертають success: true/false, інші code: 0/200
			// Будемо вважати, що якщо немає success==false або code != 0/200, то все ок
			if genericResponse.Success == false && genericResponse.Code != 0 && genericResponse.Code != 200 { // MEXC може не мати поля success
				// Можливо, відповідь була просто даними без обгортки success/code/data
			}
			if genericResponse.Code != 0 && genericResponse.Code != 200 && genericResponse.Msg != "" { // Приклад з їх документації
				return fmt.Errorf("API MEXC (%s) повернуло помилку: code %d", url, genericResponse.Code)
			}
			// Якщо є поле data, розпаковуємо його
			if len(genericResponse.Data) > 0 && string(genericResponse.Data) != "null" {
				if err := json.Unmarshal(genericResponse.Data, target); err != nil {
					return fmt.Errorf("декодування поля 'data' від %s: %w", url, err)
				}
				return nil
			}
		}
		// Якщо не розпарсилося як MEXCAPIResponseSingle або немає поля 'data',
		// пробуємо розпарсити напряму в target (деякі ендпоінти MEXC так роблять)
		if err := json.Unmarshal(rawResponse, target); err != nil {
			return fmt.Errorf("декодування прямої відповіді від %s: %w", url, err)
		}
		return nil

	} else { // targetIsList == true (для /detail)
		var genericResponseList MEXCAPIResponseList
		if err := json.Unmarshal(rawResponse, &genericResponseList); err == nil {
			if genericResponseList.Success == false && genericResponseList.Code != 0 && genericResponseList.Code != 200 {
				//
			}
			if genericResponseList.Code != 0 && genericResponseList.Code != 200 && genericResponseList.Msg != "" {
				return fmt.Errorf("API MEXC (%s) повернуло помилку: code %d", url, genericResponseList.Code)
			}

			var items []interface{}
			// Розпаковуємо кожен елемент з Data
			for _, rawItem := range genericResponseList.Data {
				// Потрібно створити новий екземпляр типу, на який вказує target
				// Це складно зробити універсально тут, простіше очікувати, що /detail поверне просто масив
			}
		}
		// /detail зазвичай повертає просто масив об'єктів [{...},{...}]
		if err := json.Unmarshal(rawResponse, target); err != nil {
			return fmt.Errorf("декодування прямого масиву від %s: %w", url, err)
		}
		return nil
	}
}

// GetFundingRates отримує ставки фінансування для USDT-M контрактів з MEXC
func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	log.Println("MEXC: Початок отримання даних про ставки фінансування...")

	var allContracts []MEXCContractDetail
	contractsURL := mexcAPIEndpointBase + allContractsPath
	// Для /detail відповідь - це просто масив, тому targetIsList = true, але fetchMEXCData очікує обгортку
	// Простіше буде обробити цей запит окремо або модифікувати fetchMEXCData
	// Поки що зробимо окремий запит для /detail
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

			// 1. Отримати Fair Price (Mark Price)
			var fairPriceInfo MEXCFairPriceInfo
			fairPriceURL := mexcAPIEndpointBase + fairPricePath + s
			if err := fetchMEXCData(fairPriceURL, &fairPriceInfo, false); err != nil {
				log.Printf("MEXC: Помилка отримання fair_price для %s: %v", s, err)
				return
			}
			if fairPriceInfo.Symbol == "" { // Перевірка, чи дані справді отримані
				// log.Printf("MEXC: Не знайдено fair_price для %s", s)
				return
			}

			// 2. Отримати Funding Rate
			var fundingRateInfo MEXCFundingRateInfo
			fundingRateURL := mexcAPIEndpointBase + fundingRatePath + s
			if err := fetchMEXCData(fundingRateURL, &fundingRateInfo, false); err != nil {
				log.Printf("MEXC: Помилка отримання funding_rate для %s: %v", s, err)
				return
			}
			if fundingRateInfo.Symbol == "" { // Перевірка
				// log.Printf("MEXC: Не знайдено funding_rate для %s", s)
				return
			}

			fundingRatePercent := fundingRateInfo.FundingRate * 100 // API MEXC вже повертає як десяткове число
			nextFundingTime := time.Unix(0, fundingRateInfo.NextFundingTime*int64(time.Millisecond)).UTC()

			// MEXC символи у форматі BTC_USDT, конвертуємо у BTCUSDT
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
