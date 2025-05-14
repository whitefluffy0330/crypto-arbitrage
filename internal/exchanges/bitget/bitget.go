package bitget

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges"
)

const (
	bitgetAPIEndpointBase = "https://api.bitget.com"
	tickersPathV2         = "/api/v2/mix/market/tickers"
	productTypeUSDT       = "USDT-FUTURES" // Може бути "umcbl" або "USDT-PERPETUAL" - треба уточнити в док. Bitget
	topNByVolume          = 75
)

// BitgetTickerV2 містить поля, які нас цікавлять з відповіді API /v2/mix/market/tickers
type BitgetTickerV2 struct {
	Symbol          string `json:"symbol"`      // Наприклад, BTCUSDT (Bitget зазвичай використовує формат без "_")
	MarkPrice       string `json:"markPrice"`   // Ціна маркування
	FundingRate     string `json:"fundingRate"` // Поточна ставка фінансування (десяткове число)
	NextFundingTime string `json:"nextFundingTime"` // Час наступного фінансування (Unix ms)
	QuoteVolume     string `json:"quoteVolume"` // Обсяг торгів за 24 години в котирувальній валюті (USDT)
	// Додаткові поля, які можуть бути корисними або для перевірки
	LastPrice       string `json:"lastPr"`      // Остання ціна
	IndexPrice      string `json:"indexPrice"`  // Ціна індексу
}

// BitgetAPIResponseV2 структура для загальної відповіді API
type BitgetAPIResponseV2 struct {
	Code        string            `json:"code"` // "00000" означає успіх
	Msg         string            `json:"msg"`
	RequestTime int64             `json:"requestTime"`
	Data        []BitgetTickerV2 `json:"data"`
}

// GetFundingRates отримує ставки фінансування для USDT-M контрактів з Bitget
func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	log.Println("Bitget: Початок отримання даних про ставки фінансування...")

	url := fmt.Sprintf("%s%s?productType=%s", bitgetAPIEndpointBase, tickersPathV2, productTypeUSDT)
	log.Printf("Bitget: Запит до API: %s", url)

	client := http.Client{Timeout: 20 * time.Second} // Збільшено таймаут
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Bitget: Помилка HTTP GET запиту (%s): %v", url, err)
		return nil, fmt.Errorf("HTTP GET запит до Bitget: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		log.Printf("Bitget: Помилка читання тіла відповіді: %v", errRead)
		return nil, fmt.Errorf("читання тіла відповіді Bitget: %w", errRead)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Bitget: Помилка статусу %d від %s. Тіло: %s", resp.StatusCode, url, string(bodyBytes))
		return nil, fmt.Errorf("статус %d від Bitget API %s", resp.StatusCode, url)
	}

	var apiResponse BitgetAPIResponseV2
	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		log.Printf("Bitget: Помилка декодування JSON відповіді: %v. Сира відповідь (перші 500б): %s", err, string(bodyBytes[:min(500, len(bodyBytes))]))
		return nil, fmt.Errorf("розбір JSON відповіді від Bitget: %w", err)
	}

	if apiResponse.Code != "00000" {
		log.Printf("Bitget: API повернуло помилку: Code=%s, Msg=%s", apiResponse.Code, apiResponse.Msg)
		return nil, fmt.Errorf("API Bitget повернуло помилку: %s (код %s)", apiResponse.Msg, apiResponse.Code)
	}

	if len(apiResponse.Data) == 0 {
		log.Println("Bitget: API повернуло успіх, але масив 'data' з тікерами порожній.")
		return []exchanges.UnifiedFundingRateInfo{}, nil
	}

	log.Printf("Bitget: Отримано %d тікерів.", len(apiResponse.Data))

	var usdtSwapTickers []BitgetTickerV2
	for _, ticker := range apiResponse.Data {
		// Bitget символи для USDT-M зазвичай мають суфікс USDT (напр. BTCUSDT)
		// І productType вже має бути USDT-FUTURES (або аналог)
		// Додатково перевіримо, чи є обсяг та чи символ закінчується на USDT
		// Деякі символи можуть бути, наприклад, BTCUSD_PERP, а не BTCUSDT
		// Для Bitget, USDT-M зазвичай мають "USDT" в кінці, наприклад, "BTCUSDT", "ETHUSDT"
		// Іноді Bitget може повертати символи типу "BTCUSDT_UMCBL" - перевіримо документацію або сиру відповідь
		// Якщо productType=USDT-FUTURES, то всі символи мають бути релевантними USDT-M.
		// Головне - перевірити наявність QuoteVolume
		if ticker.QuoteVolume != "" && strings.HasSuffix(ticker.Symbol, "USDT") { // Додамо перевірку на суфікс USDT для надійності
			usdtSwapTickers = append(usdtSwapTickers, ticker)
		}
	}
	log.Printf("Bitget: Відфільтровано %d USDT тікерів з обсягом.", len(usdtSwapTickers))


	sort.SliceStable(usdtSwapTickers, func(i, j int) bool {
		volI, _ := strconv.ParseFloat(usdtSwapTickers[i].QuoteVolume, 64)
		volJ, _ := strconv.ParseFloat(usdtSwapTickers[j].QuoteVolume, 64)
		return volI > volJ 
	})

	var symbolsToProcess []BitgetTickerV2
	if len(usdtSwapTickers) > topNByVolume {
		log.Printf("Bitget: Обмежуємо обробку до топ-%d з %d знайдених USDT тікерів за обсягом.", topNByVolume, len(usdtSwapTickers))
		symbolsToProcess = usdtSwapTickers[:topNByVolume]
	} else {
		log.Printf("Bitget: Знайдено %d USDT тікерів для обробки (менше або дорівнює ліміту %d).", len(usdtSwapTickers), topNByVolume)
		symbolsToProcess = usdtSwapTickers
	}

	if len(symbolsToProcess) == 0 {
		log.Println("Bitget: Не знайдено USDT тікерів для запиту ставок фінансування після фільтрації.")
		return []exchanges.UnifiedFundingRateInfo{}, nil
	}

	var fundingData []exchanges.UnifiedFundingRateInfo
	logCounter := 0 
	for _, item := range symbolsToProcess {
		// ---- ТИМЧАСОВИЙ ЛОГ ДЛЯ ДІАГНОСТИКИ BITGET ----
		if logCounter < 10 { // Логуємо перші 10 тікерів, що йдуть на обробку
			log.Printf("Bitget_DEBUG: Обробка тікера: Symbol=%s, MarkPrice='%s', FundingRate='%s', NextFundingTime='%s', QuoteVolume='%s', LastPrice='%s', IndexPrice='%s'", 
				item.Symbol, item.MarkPrice, item.FundingRate, item.NextFundingTime, item.QuoteVolume, item.LastPrice, item.IndexPrice)
			logCounter++
		}
		// ---- КІНЕЦЬ ТИМЧАСОВОГО ЛОГУ ----

		if item.MarkPrice == "" || item.FundingRate == "" || item.NextFundingTime == "" || item.MarkPrice == "0" || item.FundingRate == "0" && item.NextFundingTime == "0" {
			// Додамо більш детальне логування пропуску
			// if logCounter < 20 { // Логуємо ще трохи пропусків
			// 	log.Printf("Bitget_DEBUG: Пропуск символу %s через порожні або нульові критичні поля (MarkPrice: '%s', FundingRate: '%s', NextFundingTime: '%s')", item.Symbol, item.MarkPrice, item.FundingRate, item.NextFundingTime)
			// 	logCounter++
			// }
			continue
		}

		markPrice, errMP := strconv.ParseFloat(item.MarkPrice, 64)
		if errMP != nil {
			// log.Printf("Bitget: Помилка парсингу MarkPrice для %s ('%s'): %v. Пропускаємо.", item.Symbol, item.MarkPrice, errMP)
			continue
		}

		fundingRateRaw, errFR := strconv.ParseFloat(item.FundingRate, 64)
		if errFR != nil {
			// log.Printf("Bitget: Помилка парсингу FundingRate для %s ('%s'): %v. Пропускаємо.", item.Symbol, item.FundingRate, errFR)
			continue
		}
		fundingRatePercent := fundingRateRaw * 100

		nextFundingTimeMs, errNFT := strconv.ParseInt(item.NextFundingTime, 10, 64)
		if errNFT != nil {
			// log.Printf("Bitget: Помилка парсингу NextFundingTime для %s ('%s'): %v. Пропускаємо.", item.Symbol, item.NextFundingTime, errNFT)
			continue
		}
		// Якщо NextFundingTimeMs == 0, це може бути проблемою, але time.Unix(0,0) дасть 1970-01-01
		// Обробка "N/A" для невалідного часу вже є в handler.go
		nextFundingTime := time.Unix(0, nextFundingTimeMs*int64(time.Millisecond)).UTC()
		
		symbolClean := item.Symbol // Bitget символи вже у форматі BTCUSDT

		fundingData = append(fundingData, exchanges.UnifiedFundingRateInfo{
			Exchange:        "Bitget",
			Symbol:          symbolClean,
			MarkPrice:       markPrice,
			LastFundingRate: fundingRatePercent,
			NextFundingTime: nextFundingTime,
		})
	}

	log.Printf("Bitget: Успішно оброблено та зібрано дані фінансування для %d USDT пар (з %d відфільтрованих за обсягом).", len(fundingData), len(symbolsToProcess))
	return fundingData, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
