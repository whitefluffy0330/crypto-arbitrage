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
	// Використовуємо API v2 для ф'ючерсів
	tickersPathV2   = "/api/v2/mix/market/tickers"
	productTypeUSDT = "USDT-FUTURES" // Або "umcbl" - потрібно перевірити точне значення для USDT Perpetual
	topNByVolume    = 75             // Обмеження для обробки за обсягом
)

// BitgetTickerV2 містить поля, які нас цікавлять з відповіді API /v2/mix/market/tickers
type BitgetTickerV2 struct {
	Symbol          string `json:"symbol"`          // Наприклад, BTCUSDT
	MarkPrice       string `json:"markPrice"`       // Ціна маркування
	FundingRate     string `json:"fundingRate"`     // Поточна ставка фінансування (десяткове число)
	NextFundingTime string `json:"nextFundingTime"` // Час наступного фінансування (Unix ms)
	QuoteVolume     string `json:"quoteVolume"`     // Обсяг торгів за 24 години в котирувальній валюті (USDT)
	// Можуть бути й інші корисні поля: lastPr, indexPrice, openInterestValue
}

// BitgetAPIResponseV2 структура для загальної відповіді API (якщо є обгортка)
// Bitget V2 часто повертає { "code": "00000", "msg": "success", "requestTime": ..., "data": [...] }
type BitgetAPIResponseV2 struct {
	Code        string           `json:"code"` // "00000" означає успіх
	Msg         string           `json:"msg"`
	RequestTime int64            `json:"requestTime"`
	Data        []BitgetTickerV2 `json:"data"`
}

// GetFundingRates отримує ставки фінансування для USDT-M контрактів з Bitget
func GetFundingRates() ([]exchanges.UnifiedFundingRateInfo, error) {
	log.Println("Bitget: Початок отримання даних про ставки фінансування...")

	// Формуємо URL з параметром productType
	url := fmt.Sprintf("%s%s?productType=%s", bitgetAPIEndpointBase, tickersPathV2, productTypeUSDT)
	log.Printf("Bitget: Запит до API: %s", url)

	client := http.Client{Timeout: 15 * time.Second}
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

	if apiResponse.Code != "00000" { // "00000" означає успіх для Bitget API v2
		log.Printf("Bitget: API повернуло помилку: Code=%s, Msg=%s", apiResponse.Code, apiResponse.Msg)
		return nil, fmt.Errorf("API Bitget повернуло помилку: %s (код %s)", apiResponse.Msg, apiResponse.Code)
	}

	if len(apiResponse.Data) == 0 {
		log.Println("Bitget: API повернуло успіх, але масив 'data' з тікерами порожній.")
		return []exchanges.UnifiedFundingRateInfo{}, nil
	}

	log.Printf("Bitget: Отримано %d тікерів.", len(apiResponse.Data))

	// Фільтруємо та сортуємо за обсягом
	var usdtSwapTickers []BitgetTickerV2
	for _, ticker := range apiResponse.Data {
		// Bitget символи для USDT-M зазвичай мають суфікс USDT (напр. BTCUSDT)
		// І productType вже має бути USDT-FUTURES (або аналог)
		// Додатково перевіримо, чи є обсяг
		if strings.HasSuffix(ticker.Symbol, "USDT") && ticker.QuoteVolume != "" { // Bitget використовує "USDT" в кінці для USDT-M
			usdtSwapTickers = append(usdtSwapTickers, ticker)
		}
	}

	sort.SliceStable(usdtSwapTickers, func(i, j int) bool {
		volI, _ := strconv.ParseFloat(usdtSwapTickers[i].QuoteVolume, 64)
		volJ, _ := strconv.ParseFloat(usdtSwapTickers[j].QuoteVolume, 64)
		return volI > volJ // Сортування за спаданням обсягу
	})

	var symbolsToProcess []BitgetTickerV2
	if len(usdtSwapTickers) > topNByVolume {
		log.Printf("Bitget: Обмежуємо обробку до топ-%d з %d знайдених USDT-SWAP тікерів за обсягом.", topNByVolume, len(usdtSwapTickers))
		symbolsToProcess = usdtSwapTickers[:topNByVolume]
	} else {
		log.Printf("Bitget: Знайдено %d USDT-SWAP тікерів для обробки.", len(usdtSwapTickers))
		symbolsToProcess = usdtSwapTickers
	}

	if len(symbolsToProcess) == 0 {
		log.Println("Bitget: Не знайдено USDT-SWAP інструментів для запиту ставок фандингу після фільтрації.")
		return []exchanges.UnifiedFundingRateInfo{}, nil
	}

	var fundingData []exchanges.UnifiedFundingRateInfo
	for _, item := range symbolsToProcess {
		// Перевіряємо, чи є необхідні поля, перш ніж парсити
		if item.MarkPrice == "" || item.FundingRate == "" || item.NextFundingTime == "" {
			// log.Printf("Bitget: Пропуск символу %s через порожні поля (MarkPrice: '%s', FundingRate: '%s', NextFundingTime: '%s')", item.Symbol, item.MarkPrice, item.FundingRate, item.NextFundingTime)
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
		fundingRatePercent := fundingRateRaw * 100 // Конвертуємо у відсотки

		nextFundingTimeMs, errNFT := strconv.ParseInt(item.NextFundingTime, 10, 64)
		if errNFT != nil {
			// log.Printf("Bitget: Помилка парсингу NextFundingTime для %s ('%s'): %v. Пропускаємо.", item.Symbol, item.NextFundingTime, errNFT)
			continue
		}
		nextFundingTime := time.Unix(0, nextFundingTimeMs*int64(time.Millisecond)).UTC()

		// Bitget символи вже у форматі BTCUSDT
		symbolClean := item.Symbol

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

// Допоміжна функція min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
