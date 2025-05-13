package mexc

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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
	FundingRate     float64 `json:"fundingRate"`
	NextFundingTime int64   `json:"nextFundingTime"`
}

type MEXCFairPriceInfo struct {
	Symbol    string  `json:"symbol"`
	FairPrice float64 `json:"fairPrice"`
}

type MEXCAPIResponseSingle struct {
	Success bool            `json:"success,omitempty"` // omitempty, якщо поля може не бути
	Code    int             `json:"code,omitempty"`    // omitempty, якщо поля може не бути
	Msg     string          `json:"msg,omitempty"`     // ДОДАНО omitempty
	Data    json.RawMessage `json:"data"`
}

type MEXCAPIResponseList struct {
	Success bool              `json:"success,omitempty"`
	Code    int               `json:"code,omitempty"`
	Msg     string            `json:"msg,omitempty"` // ДОДАНО omitempty
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
		// Спробуємо розпарсити як обгортку
		if errUnmarshalWrapper := json.Unmarshal(rawResponse, &genericResponse); errUnmarshalWrapper == nil {
			// Перевіряємо код помилки, якщо він є
			if genericResponse.Code != 0 && genericResponse.Code != 200 && genericResponse.Msg != "" {
				return fmt.Errorf("API MEXC (%s) повернуло помилку: %s (код %d)", url, genericResponse.Msg, genericResponse.Code)
			}
			// Якщо є поле data, розпаковуємо його
			if len(genericResponse.Data) > 0 && string(genericResponse.Data) != "null" {
				if err := json.Unmarshal(genericResponse.Data, target); err != nil {
					return fmt.Errorf("декодування поля 'data' від %s: %w", url, err)
				}
				return nil
			}
		}
		// Якщо не розпарсилося як обгортка або немає поля 'data',
		// пробуємо розпарсити напряму в target
		if err := json.Unmarshal(rawResponse, target); err != nil {
			return fmt.Errorf("декодування прямої відповіді від %s: %w (після невдалої спроби обгортки)", url, err)
		}
		return nil
	} else { // targetIsList == true
		// Для списків, API /detail зазвичай повертає просто масив об'єктів [{...},{...}]
		// або іноді обгортку з полем "data", що містить масив.
		var genericResponseList MEXCAPIResponseList
		if errUnmarshalWrapper := json.Unmarshal(rawResponse, &genericResponseList); errUnmarshalWrapper == nil {
			if genericResponseList.Code != 0 && genericResponseList.Code != 200 && genericResponseList.Msg != "" {
				return fmt.Errorf("API MEXC (%s) для списку повернуло помилку: %s (код %d)", url, genericResponseList.Msg, genericResponseList.Code)
			}
			if len(genericResponseList.Data) > 0 {
				// Потрібно розпакувати масив json.RawMessage в цільовий зріз
				// Створюємо тимчасовий зріз того ж типу, що й target (який має бути *[]SomeStruct)
				// Це складно зробити універсально тут, тому спрощуємо:
				// Припускаємо, що target - це *[]MEXCContractDetail для /detail
				// І що genericResponseList.Data містить масив цих структур
				// Для цього потрібно, щоб target був правильного типу для json.Unmarshal(genericResponseList.Data, target)
				// Або ж ми розпаковуємо genericResponseList.Data в []json.RawMessage, а потім кожен елемент окремо.
				// Найпростіше - очікувати, що /detail повертає просто масив, або обгортку, яку ми розпакуємо.

				// Оскільки /detail повертає просто масив, наступний json.Unmarshal(rawResponse, target) має спрацювати.
				// ВИДАЛЕНО НЕЗАВЕРШЕНИЙ БЛОК З items та rawItem
			}
		}
		// Припускаємо, що /detail повертає просто масив об'єктів, або обгортку, яку ми вже перевірили
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
	// MEXC /detail повертає масив напряму, без обгортки code/msg/data
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
				log.Printf("MEXC: Помилка отримання fair_price для %s: %v", s, err)
				return
			}
			if fairPriceInfo.Symbol == "" {
				// log.Printf("MEXC: Не знайдено fair_price для %s (або порожня відповідь)", s)
				return
			}

			var fundingRateInfo MEXCFundingRateInfo
			fundingRateURL := mexcAPIEndpointBase + fundingRatePath + s
			if err := fetchMEXCData(fundingRateURL, &fundingRateInfo, false); err != nil {
				log.Printf("MEXC: Помилка отримання funding_rate для %s: %v", s, err)
				return
			}
			 if fundingRateInfo.Symbol == "" {
                // log.Printf("MEXC: Не знайдено funding_rate для %s (або порожня відповідь)", s)
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
