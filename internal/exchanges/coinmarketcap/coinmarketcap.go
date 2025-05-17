package coinmarketcap

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges" // Можливо, для уніфікованих структур
)

const (
	cmcAPIEndpointBase = "https://pro-api.coinmarketcap.com"
	listingsLatestPath = "/v1/cryptocurrency/listings/latest"
	marketPairsPath    = "/v1/cryptocurrency/market-pairs/latest" // Для отримання тікерів монети
)

// --- Структури для відповіді API CoinMarketCap ---

// CMCCoinInfo - для даних з /v1/cryptocurrency/listings/latest
type CMCCoinInfo struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
	Slug   string `json:"slug"`
	Quote  map[string]struct {
		Price     float64 `json:"price"`
		Volume24h float64 `json:"volume_24h"`
		MarketCap float64 `json:"market_cap"`
	} `json:"quote"`
}

type CMCListingsLatestResponse struct {
	Status struct {
		Timestamp    string `json:"timestamp"`
		ErrorCode    int    `json:"error_code"`
		ErrorMessage string `json:"error_message"`
		Elapsed      int    `json:"elapsed"`
		CreditCount  int    `json:"credit_count"`
	} `json:"status"`
	Data []CMCCoinInfo `json:"data"`
}

// CMCTickerInfo - для даних з /v1/cryptocurrency/market-pairs/latest
type CMCMarketPair struct {
	Exchange struct {
		Name string `json:"name"`
		Slug string `json:"slug"` // Ідентифікатор біржі
		ID   int    `json:"id"`
	} `json:"exchange"`
	MarketPair  string `json:"market_pair"` // Наприклад "BTC/USDT"
	BaseSymbol  string `json:"base_symbol"`
	QuoteSymbol string `json:"quote_symbol"`
	Quote       map[string]struct {
		Price     float64 `json:"price"`
		Volume24h float64 `json:"volume_24h"`
	} `json:"quote"`
	// Можливо, є поле для "Liquidity Score" або аналог "Trust Score"
	// Потрібно буде перевірити повну відповідь API
	EffectiveLiquidity *float64 `json:"effective_liquidity,omitempty"` // Приклад, якщо є
	MarketScore        *float64 `json:"market_score,omitempty"`        // Приклад
}

type CMCMarketPairsResponse struct {
	Status struct {
		Timestamp    string `json:"timestamp"`
		ErrorCode    int    `json:"error_code"`
		ErrorMessage string `json:"error_message"`
	} `json:"status"`
	Data struct {
		ID             int             `json:"id"`
		Name           string          `json:"name"`
		Symbol         string          `json:"symbol"`
		NumMarketPairs int             `json:"num_market_pairs"`
		MarketPairs    []CMCMarketPair `json:"market_pairs"`
	} `json:"data"`
}

// Уніфіковані структури (можна буде винести в спільний пакет)
type UnifiedCoinInfoCMC struct {
	ID     string // Для CMC це може бути числовий ID або slug
	Symbol string
	Name   string
}

type UnifiedTickerInfoCMC struct {
	ExchangeName       string
	ExchangeIdentifier string // slug з CMC
	BaseCurrency       string
	QuoteCurrency      string
	PriceUSD           float64
	Volume24hUSD       float64
	TrustScore         string // Поки що N/A, або спробуємо використати Liquidity Score
	TradeURL           string // CMC може не надавати прямі TradeURL
}

func makeCMCRequest(apiKey, endpointPath string, queryParams url.Values) ([]byte, error) {
	reqURL := fmt.Sprintf("%s%s", cmcAPIEndpointBase, endpointPath)
	if queryParams != nil {
		reqURL += "?" + queryParams.Encode()
	}

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("створення запиту до CMC: %w", err)
	}
	req.Header.Set("Accepts", "application/json")
	req.Header.Set("X-CMC_PRO_API_KEY", apiKey)

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP запит до CMC (%s): %w", endpointPath, err)
	}
	defer resp.Body.Close()

	bodyBytes, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		return nil, fmt.Errorf("читання тіла відповіді від CMC (%s): %w", endpointPath, errRead)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("CoinMarketCap: Помилка статусу %d від %s. Тіло: %s", resp.StatusCode, reqURL, string(bodyBytes))
		// Спробуємо розпарсити помилку, якщо вона є в JSON форматі
		var errorResponse struct {
			Status struct {
				ErrorCode    int    `json:"error_code"`
				ErrorMessage string `json:"error_message"`
			} `json:"status"`
		}
		if json.Unmarshal(bodyBytes, &errorResponse) == nil && errorResponse.Status.ErrorCode != 0 {
			return nil, fmt.Errorf("API CoinMarketCap (%s) повернуло помилку: %s (код %d)", endpointPath, errorResponse.Status.ErrorMessage, errorResponse.Status.ErrorCode)
		}
		return nil, fmt.Errorf("статус %d від CoinMarketCap API (%s)", resp.StatusCode, endpointPath)
	}
	return bodyBytes, nil
}

// GetTopMarketCapCoinsCMC отримує топ-N монет з CoinMarketCap
func GetTopMarketCapCoinsCMC(apiKey string, limit int) ([]UnifiedCoinInfoCMC, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API ключ CoinMarketCap не надано")
	}
	if limit <= 0 {
		limit = 50
	} // За замовчуванням

	params := url.Values{}
	params.Add("start", "1")
	params.Add("limit", strconv.Itoa(limit))
	params.Add("convert", "USD")
	// Можна додати sort=market_cap, sort_dir=desc, хоча це зазвичай типово

	log.Printf("CoinMarketCap: Запит топ-%d монет.", limit)
	bodyBytes, err := makeCMCRequest(apiKey, listingsLatestPath, params)
	if err != nil {
		return nil, err
	}

	var cmcResponse CMCListingsLatestResponse
	if err := json.Unmarshal(bodyBytes, &cmcResponse); err != nil {
		return nil, fmt.Errorf("декодування відповіді /listings/latest від CMC: %w. Body: %s", err, string(bodyBytes))
	}

	if cmcResponse.Status.ErrorCode != 0 {
		return nil, fmt.Errorf("API CMC /listings/latest помилка: %s (код %d)", cmcResponse.Status.ErrorMessage, cmcResponse.Status.ErrorCode)
	}

	var coins []UnifiedCoinInfoCMC
	for _, coinData := range cmcResponse.Data {
		coins = append(coins, UnifiedCoinInfoCMC{
			ID:     strconv.Itoa(coinData.ID), // Використовуємо числовий ID як рядок
			Symbol: strings.ToUpper(coinData.Symbol),
			Name:   coinData.Name,
		})
	}
	log.Printf("CoinMarketCap: Отримано %d монет.", len(coins))
	return coins, nil
}

// GetCoinTickersCMC отримує тікери для конкретної монети з CoinMarketCap
func GetCoinTickersCMC(apiKey string, coinIdentifier string) ([]UnifiedTickerInfoCMC, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API ключ CoinMarketCap не надано")
	}

	params := url.Values{}
	// CoinMarketCap дозволяє шукати за ID, slug або symbol.
	// Якщо coinIdentifier - це число, вважаємо, що це ID. Інакше - symbol.
	if _, err := strconv.Atoi(coinIdentifier); err == nil {
		params.Add("id", coinIdentifier)
	} else {
		params.Add("symbol", strings.ToUpper(coinIdentifier))
	}
	params.Add("convert", "USD")
	params.Add("limit", "100") // За замовчуванням 100, макс 5000 на деяких планах

	log.Printf("CoinMarketCap: Запит тікерів для монети '%s'.", coinIdentifier)
	bodyBytes, err := makeCMCRequest(apiKey, marketPairsPath, params)
	if err != nil {
		return nil, err
	}

	var cmcResponse CMCMarketPairsResponse
	if err := json.Unmarshal(bodyBytes, &cmcResponse); err != nil {
		return nil, fmt.Errorf("декодування відповіді /market-pairs/latest від CMC: %w. Body: %s", err, string(bodyBytes))
	}

	if cmcResponse.Status.ErrorCode != 0 {
		return nil, fmt.Errorf("API CMC /market-pairs/latest помилка: %s (код %d)", cmcResponse.Status.ErrorMessage, cmcResponse.Status.ErrorCode)
	}

	var tickers []UnifiedTickerInfoCMC
	for _, pair := range cmcResponse.Data.MarketPairs {
		if priceData, ok := pair.Quote["USD"]; ok {
			// Визначаємо Base та Quote з market_pair
			pairParts := strings.Split(pair.MarketPair, "/")
			var base, quote string
			if len(pairParts) == 2 {
				base = pairParts[0]
				quote = pairParts[1]
			} else {
				base = pair.BaseSymbol // Запасний варіант
				quote = pair.QuoteSymbol
			}

			// Спроба отримати Trust Score (або аналог)
			// CMC не має прямого "green/yellow/red" Trust Score для бірж у цьому ендпоінті.
			// Вони мають "Liquidity Score" для пар, але він не завжди є.
			// Поки що залишимо TrustScore порожнім.
			trustScoreDisplay := "N/A"

			tickers = append(tickers, UnifiedTickerInfoCMC{
				ExchangeName:       pair.Exchange.Name,
				ExchangeIdentifier: pair.Exchange.Slug, // Використовуємо slug як ідентифікатор
				BaseCurrency:       strings.ToUpper(base),
				QuoteCurrency:      strings.ToUpper(quote),
				PriceUSD:           priceData.Price,
				Volume24hUSD:       priceData.Volume24h,
				TrustScore:         trustScoreDisplay,
				// TradeURL: CoinMarketCap зазвичай не надає прямих trade_url у цьому ендпоінті
			})
		}
	}
	log.Printf("CoinMarketCap: Отримано %d тікерів для %s.", len(tickers), cmcResponse.Data.Name)
	return tickers, nil
}
