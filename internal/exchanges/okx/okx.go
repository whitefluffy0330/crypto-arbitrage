package okx

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort" // Потрібен для сортування
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges"
)

const (
	okxAPIEndpoint        = "https://www.okx.com"
	instrumentsPathV5     = "/api/v5/public/instruments" // Можливо, вже не потрібен, якщо /market/tickers дає все
	fundingRatePathV5     = "/api/v5/public/funding-rate"
	markPricePathV5       = "/api/v5/public/mark-price"
	tickersPathV5         = "/api/v5/market/tickers"      // Будемо використовувати цей для отримання списку та обсягів
	maxConcurrentRequests = 5                             // Зменшено для OKX теж
	maxErrorLogs        = 5
	topNByVolumeOKX     = 75 // Обмеження для обробки за обсягом
)

type OKXTickerInfo struct { // Для /api/v5/market/tickers
	InstType      string `json:"instType"`
	InstID        string `json:"instId"`
	MarkPx        string `json:"markPx"`        // Ціна маркування з тікера (якщо є)
	FundingRate   string `json:"fundingRate"`   // Ставка фінансування з тікера (якщо є)
	NextFundingTime string `json:"nextFundingTime"` // Час наступного фінансування з тікера (якщо є)
	VolCcy24h     string `json:"volCcy24h"`     // Обсяг в котирувальній валюті за 24 години (наприклад, USDT)
	Vol24h        string `json:"vol24h"`        // Обсяг в базовій валюті за 24 години
	// ... інші поля з тікера ...
}

type OKXMarkPriceInfoAPI struct { // Для /api/v5/public/mark-price
	InstType string `json:"instType"`
	InstID   string `json:"instId"`
	MarkPx   string `json:"markPx"`
	Ts       string `json:"ts"`
}

type OKXFundingRateInfoAPI struct { // Для /api/v5/public/funding-rate
	InstType      string `json:"instType"`
	InstID        string `json:"instId"`
	FundingRate   string `json:"fundingRate"`
	NextFundingRate string `json:"nextFundingRate"`
	FundingTime   string `json:"fundingTime"`
}

type OKXAPIResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func fetchOKXData(url string, target interface{}) error {
	client := http.Client{Timeout: 20 * time.Second} // Збільшено таймаут
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
		log.Printf("OKX: Помилка статусу %d від %s. Тіло: %s", resp.StatusCode, url, string(bodyBytes))
		return fmt.Errorf("статус %d від %s", resp.StatusCode, url)
	}

	var apiResponse OKXAPIResponse
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		// Якщо це не стандартна обгортка, спробуємо розпарсити напряму
		if errDirect := json.Unmarshal(bodyBytes, target); errDirect != nil {
			log.Printf("OKX: Помилка декодування відповіді від %s (пряма та обгортка): %v / %v. Сира відповідь: %s", url, errDirect, err, string(bodyBytes))
			return fmt.Errorf("декодування відповіді від %s: %w (пряма) / %v (обгортка)", url, errDirect, err)
		}
		return nil
	}

	if apiResponse.Code != "0" {
		return fmt.Errorf("API OKX (%s) повернуло помилку: %s (код %s)", url, apiResponse.Msg, apiResponse.Code)
	}

	if err := json.Unmarshal(apiResponse.Data, target); err != nil {
		return fmt.Errorf("декодування поля 'data' від %s: %w. Raw 'data': %s", url, err, string(apiResponse.Data))
	}
	return nil
}

func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	log.Println("OKX: Початок отримання даних про ставки фінансування...")

	// 1. Отримати всі тікери для SWAP, щоб відфільтрувати за обсягом
	var allTickers []OKXTickerInfo
	tickersURL := fmt.Sprintf("%s%s?instType=SWAP", okxAPIEndpoint, tickersPathV5)
	if err := fetchOKXData(tickersURL, &allTickers); err != nil {
		log.Printf("OKX: Помилка отримання списку тікерів: %v", err)
		return nil, fmt.Errorf("отримання тікерів OKX: %w", err)
	}
	log.Printf("OKX: Отримано %d SWAP тікерів.", len(allTickers))

	// Фільтруємо та сортуємо за обсягом
	var usdtSwapTickers []OKXTickerInfo
	for _, ticker := range allTickers {
		if strings.HasSuffix(ticker.InstID, "-USDT-SWAP") && ticker.VolCcy24h != "" {
			usdtSwapTickers = append(usdtSwapTickers, ticker)
		}
	}

	sort.SliceStable(usdtSwapTickers, func(i, j int) bool {
		volI, _ := strconv.ParseFloat(usdtSwapTickers[i].VolCcy24h, 64)
		volJ, _ := strconv.ParseFloat(usdtSwapTickers[j].VolCcy24h, 64)
		return volI > volJ // Сортування за спаданням обсягу
	})

	var instIDsToProcess []string
	if len(usdtSwapTickers) > topNByVolumeOKX {
		log.Printf("OKX: Обмежуємо обробку до топ-%d з %d знайдених USDT-SWAP тікерів за обсягом.", topNByVolumeOKX, len(usdtSwapTickers))
		for i := 0; i < topNByVolumeOKX; i++ {
			instIDsToProcess = append(instIDsToProcess, usdtSwapTickers[i].InstID)
		}
	} else {
		log.Printf("OKX: Знайдено %d USDT-SWAP тікерів для обробки (менше або дорівнює ліміту %d).", len(usdtSwapTickers), topNByVolumeOKX)
		for _, ticker := range usdtSwapTickers {
			instIDsToProcess = append(instIDsToProcess, ticker.InstID)
		}
	}

	if len(instIDsToProcess) == 0 {
		log.Println("OKX: Не знайдено USDT-SWAP інструментів для запиту ставок фандингу після фільтрації за обсягом.")
		return []exchanges.UnifiedFundingRateInfo{}, nil
	}
	log.Printf("OKX: Буде оброблено %d інструментів після фільтрації за обсягом.", len(instIDsToProcess))


	// 2. Отримати ціни маркування для всіх SWAP інструментів (або тільки для обраних)
	// Ефективніше отримати всі одразу, якщо можливо
	var markPricesRaw []OKXMarkPriceInfoAPI
	markPriceURL := fmt.Sprintf("%s%s?instType=SWAP", okxAPIEndpoint, markPricePathV5)
	if err := fetchOKXData(markPriceURL, &markPricesRaw); err != nil {
		log.Printf("OKX: Помилка отримання цін маркування: %v. Продовжуємо без них.", err)
		// Не повертаємо помилку, спробуємо отримати дані без цін маркування
	}
	markPricesMap := make(map[string]float64)
	for _, mp := range markPricesRaw {
		price, errParse := strconv.ParseFloat(mp.MarkPx, 64)
		if errParse == nil {
			markPricesMap[mp.InstID] = price
		}
	}
	log.Printf("OKX: Отримано %d цін маркування, %d успішно розпарсено.", len(markPricesRaw), len(markPricesMap))


	// 3. Для кожного обраного інструменту отримати ставку фінансування
	var fundingData []exchanges.UnifiedFundingRateInfo
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, maxConcurrentRequests)

	var fundingRateErrorCount int32
	var fundingRateEmptyCount int32
	var processedCount int32


	for _, instID := range instIDsToProcess {
		wg.Add(1)
		sem <- struct{}{}

		go func(instrumentID string) {
			defer wg.Done()
			defer func() { <-sem }()

			var fundingRateInfoList []OKXFundingRateInfoAPI // API повертає масив з одного елемента
			fundingRateURL := fmt.Sprintf("%s%s?instId=%s", okxAPIEndpoint, fundingRatePathV5, instrumentID)
			
			if err := fetchOKXData(fundingRateURL, &fundingRateInfoList); err != nil {
				mu.Lock()
				if fundingRateErrorCount < maxErrorLogs {
					log.Printf("OKX: Помилка отримання ставки фандингу для %s: %v", instrumentID, err)
				}
				fundingRateErrorCount++
				mu.Unlock()
				return
			}

			if len(fundingRateInfoList) == 0 || fundingRateInfoList[0].FundingRate == "" || fundingRateInfoList[0].FundingTime == "" {
				mu.Lock()
				if fundingRateEmptyCount < maxErrorLogs {
					// log.Printf("OKX: Немає даних фандингу або порожні поля для %s", instrumentID)
				}
				fundingRateEmptyCount++
				mu.Unlock()
				return
			}
			
			fundingItem := fundingRateInfoList[0]

			fundingRateRaw, errFR := strconv.ParseFloat(fundingItem.FundingRate, 64)
			if errFR != nil {
				// Вже логується у fetchOKXData, якщо парсинг там
				return
			}
			fundingRatePercent := fundingRateRaw * 100

			nextFundingTimeMs, errNFT := strconv.ParseInt(fundingItem.FundingTime, 10, 64)
			if errNFT != nil {
				return
			}
			nextFundingTime := time.Unix(0, nextFundingTimeMs*int64(time.Millisecond)).UTC()

			markPrice := 0.0
			if mp, ok := markPricesMap[instrumentID]; ok {
				markPrice = mp
			}
			
			symbolClean := strings.Replace(instrumentID, "-SWAP", "", 1)
			symbolClean = strings.Replace(symbolClean, "-", "", 1)

			mu.Lock()
			fundingData = append(fundingData, exchanges.UnifiedFundingRateInfo{
				Exchange:        "OKX",
				Symbol:          symbolClean,
				MarkPrice:       markPrice,
				LastFundingRate: fundingRatePercent,
				NextFundingTime: nextFundingTime,
			})
			processedCount++
			mu.Unlock()

		}(instID)
	}

	wg.Wait()

	log.Printf("OKX: Успішно оброблено та зібрано дані фінансування для %d з %d USDT SWAP пар. Помилок FundingRate: %d (з них порожніх: %d).",
		processedCount, len(instIDsToProcess), fundingRateErrorCount, fundingRateEmptyCount)
	return fundingData, nil
}

// min функція не використовується
