package okx

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync" // Потрібен для go-рутин та WaitGroup
	"time"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges" // Для UnifiedFundingRateInfo
)

const (
	okxAPIEndpoint        = "https://www.okx.com"
	instrumentsPathV5     = "/api/v5/public/instruments"
	fundingRatePathV5     = "/api/v5/public/funding-rate"
	markPricePathV5       = "/api/v5/public/mark-price"
	maxConcurrentRequests = 10 // Обмеження одночасних запитів до API OKX
)

// --- Структури для відповіді API OKX ---

// OKXInstrumentInfo для /api/v5/public/instruments
type OKXInstrumentInfo struct {
	InstType  string `json:"instType"`  // SWAP, FUTURES, OPTION, SPOT, MARGIN
	InstID    string `json:"instId"`    // BTC-USDT-SWAP
	Uly       string `json:"uly"`       // BTC-USDT (базовий актив)
	SettleCcy string `json:"settleCcy"` // USDT, BTC (валюта розрахунку)
	State     string `json:"state"`     // live, suspended, preopen
}

// OKXMarkPriceInfo для /api/v5/public/mark-price
type OKXMarkPriceInfo struct {
	InstType string `json:"instType"`
	InstID   string `json:"instId"`
	MarkPx   string `json:"markPx"` // Ціна маркування
	Ts       string `json:"ts"`     // Час оновлення
}

// OKXFundingRateInfo для /api/v5/public/funding-rate
type OKXFundingRateInfo struct {
	InstType      string `json:"instType"`
	InstID        string `json:"instId"`
	FundingRate   string `json:"fundingRate"`     // Поточна ставка
	NextFundingRate string `json:"nextFundingRate"` // Очікувана ставка
	FundingTime   string `json:"fundingTime"`     // Час наступної виплати (UTC ms)
}

// OKXAPIResponse структура для загальної відповіді API
type OKXAPIResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"` // Використовуємо RawMessage для гнучкого парсингу
}

// --- Функції для роботи з API ---

// fetchOKXData робить GET-запит та розпаковує відповідь у надану структуру
func fetchOKXData(url string, target interface{}) error {
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET до %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Спробуємо прочитати тіло помилки
		// bodyBytes, _ := io.ReadAll(resp.Body)
		// log.Printf("OKX: Тіло відповіді при помилці статусу для %s: %s", url, string(bodyBytes))
		return fmt.Errorf("статус %d від %s", resp.StatusCode, url)
	}

	var apiResponse OKXAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return fmt.Errorf("декодування загальної відповіді від %s: %w", url, err)
	}

	if apiResponse.Code != "0" {
		return fmt.Errorf("API OKX (%s) повернуло помилку: %s (код %s)", url, apiResponse.Msg, apiResponse.Code)
	}

	// Розпаковуємо поле "data" у цільову структуру
	if err := json.Unmarshal(apiResponse.Data, target); err != nil {
		return fmt.Errorf("декодування поля 'data' від %s: %w", url, err)
	}
	return nil
}

// GetFundingRates отримує ставки фінансування для USDT-M SWAP контрактів з OKX
func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	log.Println("OKX: Початок отримання даних про ставки фінансування...")

	// 1. Отримати список SWAP інструментів
	var instruments []OKXInstrumentInfo
	instrumentsURL := fmt.Sprintf("%s%s?instType=SWAP", okxAPIEndpoint, instrumentsPathV5)
	if err := fetchOKXData(instrumentsURL, &instruments); err != nil {
		log.Printf("OKX: Помилка отримання списку інструментів: %v", err)
		return nil, fmt.Errorf("отримання інструментів OKX: %w", err)
	}
	log.Printf("OKX: Отримано %d SWAP інструментів.", len(instruments))

	// Фільтруємо тільки активні USDT-margined SWAP контракти
	var usdtSwapInstIDs []string
	for _, inst := range instruments {
		if inst.State == "live" && inst.SettleCcy == "USDT" && strings.HasSuffix(inst.InstID, "-USDT-SWAP") {
			usdtSwapInstIDs = append(usdtSwapInstIDs, inst.InstID)
		}
	}
	log.Printf("OKX: Знайдено %d активних USDT-SWAP інструментів.", len(usdtSwapInstIDs))
	if len(usdtSwapInstIDs) == 0 {
		return []exchanges.UnifiedFundingRateInfo{}, nil
	}

	// 2. Отримати ціни маркування для всіх SWAP інструментів
	var markPricesRaw []OKXMarkPriceInfo
	markPriceURL := fmt.Sprintf("%s%s?instType=SWAP", okxAPIEndpoint, markPricePathV5)
	if err := fetchOKXData(markPriceURL, &markPricesRaw); err != nil {
		log.Printf("OKX: Помилка отримання цін маркування: %v", err)
		// Продовжуємо, але ціни маркування можуть бути недоступні
	}
	markPricesMap := make(map[string]float64)
	for _, mp := range markPricesRaw {
		price, errParse := strconv.ParseFloat(mp.MarkPx, 64)
		if errParse == nil {
			markPricesMap[mp.InstID] = price
		}
	}
	log.Printf("OKX: Отримано %d цін маркування, %d успішно розпарсено.", len(markPricesRaw), len(markPricesMap))

	// 3. Для кожного USDT-SWAP інструменту отримати ставку фінансування
	var fundingData []exchanges.UnifiedFundingRateInfo
	var wg sync.WaitGroup
	var mu sync.Mutex // Для безпечного доступу до fundingData з горутин
	
	// Створюємо семафор для обмеження кількості одночасних запитів
	sem := make(chan struct{}, maxConcurrentRequests)

	for _, instID := range usdtSwapInstIDs {
		wg.Add(1)
		sem <- struct{}{} // Захоплюємо слот у семафорі

		go func(instrumentID string) {
			defer wg.Done()
			defer func() { <-sem }() // Звільняємо слот

			var fundingRateInfoList []OKXFundingRateInfo // API повертає масив з одного елемента
			fundingRateURL := fmt.Sprintf("%s%s?instId=%s", okxAPIEndpoint, fundingRatePathV5, instrumentID)
			
			if err := fetchOKXData(fundingRateURL, &fundingRateInfoList); err != nil {
				log.Printf("OKX: Помилка отримання ставки фандингу для %s: %v", instrumentID, err)
				return
			}

			if len(fundingRateInfoList) == 0 || fundingRateInfoList[0].FundingRate == "" || fundingRateInfoList[0].FundingTime == "" {
				// log.Printf("OKX: Немає даних фандингу або порожні поля для %s", instrumentID)
				return
			}
			
			fundingItem := fundingRateInfoList[0]

			fundingRateRaw, errFR := strconv.ParseFloat(fundingItem.FundingRate, 64)
			if errFR != nil {
				log.Printf("OKX: Помилка парсингу FundingRate для %s ('%s'): %v", instrumentID, fundingItem.FundingRate, errFR)
				return
			}
			fundingRatePercent := fundingRateRaw * 100

			nextFundingTimeMs, errNFT := strconv.ParseInt(fundingItem.FundingTime, 10, 64)
			if errNFT != nil {
				log.Printf("OKX: Помилка парсингу FundingTime для %s ('%s'): %v", instrumentID, fundingItem.FundingTime, errNFT)
				return
			}
			nextFundingTime := time.Unix(0, nextFundingTimeMs*int64(time.Millisecond)).UTC()

			markPrice := 0.0
			if mp, ok := markPricesMap[instrumentID]; ok {
				markPrice = mp
			} else {
				// log.Printf("OKX: Ціна маркування не знайдена для %s, використовується 0.0", instrumentID)
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
			mu.Unlock()

		}(instID)
	}

	wg.Wait() // Чекаємо завершення всіх горутин

	log.Printf("OKX: Успішно оброблено та зібрано дані фінансування для %d USDT SWAP пар.", len(fundingData))
	return fundingData, nil
}
