package okx

import (
	"bytes" // ДОДАНО для повторного читання тіла відповіді
	"encoding/json"
	"fmt"
	"io" // ДОДАНО для io.ReadAll
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges" // Для UnifiedFundingRateInfo
)

const (
	okxAPIEndpoint = "https://www.okx.com"
	tickersPathV5  = "/api/v5/market/tickers"
)

type OKXTickerV5 struct {
	InstType      string `json:"instType"`
	InstID        string `json:"instId"`
	MarkPx        string `json:"markPx"`
	FundingRate   string `json:"fundingRate"`
	NextFundingTime string `json:"nextFundingTime"`
	// Додамо ще кілька полів, які часто бувають у тікерах, щоб побачити, чи є вони
	LastPx        string `json:"last"` // Остання ціна
	OpenInterest  string `json:"openInt"` // Відкритий інтерес (часто називається openInterest або openInterestValue)
	Vol24h        string `json:"vol24h"`  // Обсяг за 24 години
}

type OKXAPIResponseV5 struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data []OKXTickerV5 `json:"data"`
}

func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	url := fmt.Sprintf("%s%s?instType=SWAP", okxAPIEndpoint, tickersPathV5)
	log.Printf("Запит до OKX API для Funding Rates: %s", url)

	client := http.Client{Timeout: 15 * time.Second} // Збільшимо таймаут для надійності
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Помилка HTTP GET запиту до OKX API (%s): %v", url, err)
		return nil, fmt.Errorf("помилка HTTP GET запиту до OKX: %w", err)
	}
	defer resp.Body.Close()

	// ---- ТИМЧАСОВЕ ЛОГУВАННЯ СИРОЇ ВІДПОВІДІ ----
	bodyBytes, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		log.Printf("OKX: Помилка читання тіла відповіді: %v", errRead)
		// Не повертаємо помилку тут, спробуємо декодувати, якщо щось прочиталося
	} else {
		log.Printf("OKX: Сира відповідь API (перші 1000 символів): %s", string(bodyBytes[:min(1000, len(bodyBytes))]))
		// Повертаємо тіло відповіді назад в resp.Body для подальшого декодування
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	// ---- КІНЕЦЬ ТИМЧАСОВОГО ЛОГУВАННЯ ----

	if resp.StatusCode != http.StatusOK {
		log.Printf("Помилка статусу від OKX API (%s): Status=%s, Code=%d", url, resp.Status, resp.StatusCode)
		// Спробуємо прочитати тіло помилки, якщо воно є
		errorBodyBytes, _ := io.ReadAll(io.NopCloser(bytes.NewBuffer(bodyBytes))) // Читаємо з копії, якщо вже читали
		if len(errorBodyBytes) > 0 {
			log.Printf("OKX: Тіло відповіді при помилці статусу: %s", string(errorBodyBytes))
		}
		return nil, fmt.Errorf("помилка статусу від OKX API: %s (%d)", resp.Status, resp.StatusCode)
	}

	var apiResponse OKXAPIResponseV5
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&apiResponse); err != nil {
		log.Printf("Помилка декодування JSON відповіді від OKX API: %v. Можливо, сира відповідь вище допоможе.", err)
		return nil, fmt.Errorf("помилка розбору JSON відповіді від OKX: %w", err)
	}

	if apiResponse.Code != "0" {
		log.Printf("API OKX повернуло помилку: Code=%s, Msg=%s. Дані: %v", apiResponse.Code, apiResponse.Msg, apiResponse.Data)
		return nil, fmt.Errorf("API OKX повернуло помилку: %s (код %s)", apiResponse.Msg, apiResponse.Code)
	}

	if len(apiResponse.Data) == 0 {
		log.Printf("OKX: API повернуло успіх (code 0), але масив 'data' порожній. Перевірте параметри запиту або відповідь API.")
		return []exchanges.UnifiedFundingRateInfo{}, nil // Повертаємо порожній зріз, а не помилку
	}


	var fundingData []exchanges.UnifiedFundingRateInfo
	for i, item := range apiResponse.Data {
		// Логуємо перші кілька елементів для аналізу
		if i < 5 { // Логуємо перші 5
			log.Printf("OKX: Обробка елемента #%d: %+v", i, item)
		}

		if !strings.HasSuffix(item.InstID, "-USDT-SWAP") {
			if i < 20 { // Логуємо пропуск для перших кількох, щоб не спамити
				log.Printf("OKX: Пропуск InstID '%s' (не USDT-SWAP)", item.InstID)
			}
			continue
		}
		
		if item.MarkPx == "" || item.FundingRate == "" || item.NextFundingTime == "" {
			log.Printf("OKX: Пропуск InstID '%s' через порожні поля (MarkPx: '%s', FundingRate: '%s', NextFundingTime: '%s')", item.InstID, item.MarkPx, item.FundingRate, item.NextFundingTime)
			continue
		}

		markPrice, errMP := strconv.ParseFloat(item.MarkPx, 64)
		if errMP != nil {
			log.Printf("OKX: Помилка парсингу MarkPx для %s ('%s'): %v. Пропускаємо.", item.InstID, item.MarkPx, errMP)
			continue
		}

		fundingRateRaw, errFR := strconv.ParseFloat(item.FundingRate, 64)
		if errFR != nil {
			log.Printf("OKX: Помилка парсингу FundingRate для %s ('%s'): %v. Пропускаємо.", item.InstID, item.FundingRate, errFR)
			continue
		}
		fundingRatePercent := fundingRateRaw * 100

		nextFundingTimeMs, errNFT := strconv.ParseInt(item.NextFundingTime, 10, 64)
		if errNFT != nil {
			log.Printf("OKX: Помилка парсингу NextFundingTime для %s ('%s'): %v. Пропускаємо.", item.InstID, item.NextFundingTime, errNFT)
			continue
		}
		nextFundingTime := time.Unix(0, nextFundingTimeMs*int64(time.Millisecond)).UTC()

		symbolClean := strings.Replace(item.InstID, "-SWAP", "", 1)
		symbolClean = strings.Replace(symbolClean, "-", "", 1)

		fundingData = append(fundingData, exchanges.UnifiedFundingRateInfo{
			Exchange:        "OKX",
			Symbol:          symbolClean,
			MarkPrice:       markPrice,
			LastFundingRate: fundingRatePercent,
			NextFundingTime: nextFundingTime,
		})
	}

	log.Printf("Отримано та оброблено дані фінансування для %d USDT SWAP пар з OKX.", len(fundingData))
	return fundingData, nil
}

// Допоміжна функція min для логування
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
