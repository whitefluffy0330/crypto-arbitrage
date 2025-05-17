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
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/config" // Може знадобитися для лімітів
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

// CMCMarketPair - для даних з /v1/cryptocurrency/market-pairs/latest
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
	EffectiveLiquidity *float64 `json:"effective_liquidity,omitempty"`
	MarketScore        *float64 `json:"market_score,omitempty"`
}

type CMCMarketPairsResponse struct {
	Status struct {
		Timestamp    string `json:"timestamp"`
		ErrorCode    int    `json:"error_code"`
		ErrorMessage string `json:"error_message"`
		CreditCount  int    `json:"credit_count"` // Додано для відстеження кредитів
	} `json:"status"`
	Data struct {
		ID             int             `json:"id"`
		Name           string          `json:"name"`
		Symbol         string          `json:"symbol"`
		NumMarketPairs int             `json:"num_market_pairs"`
		MarketPairs    []CMCMarketPair `json:"market_pairs"`
	} `json:"data"`
}

// Уніфіковані структури
type UnifiedCoinInfoCMC struct {
	ID     string // Для CMC це числовий ID, конвертований у рядок
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
	TrustScore         string // Заповнюватиметься як "N/A"
	TradeURL           string // Залишатиметься порожнім
}

func makeCMCRequest(apiKey, endpointPath string, queryParams url.Values) ([]byte, error) {
	reqURL := fmt.Sprintf("%s%s", cmcAPIEndpointBase, endpointPath)
	if queryParams != nil && len(queryParams) > 0 {
		reqURL += "?" + queryParams.Encode()
	}

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("створення запиту до CMC: %w", err)
	}
	req.Header.Set("Accepts", "application/json")
	req.Header.Set("X-CMC_PRO_API_KEY", apiKey)

	client := http.Client{Timeout: 20 * time.Second}
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
		var errorResponse struct {
			Status struct {
				ErrorCode    int    `json:"error_code"`
				ErrorMessage string `json:"error_message"`
			} `json:"status"`
		}
		logBody := string(bodyBytes)
		if len(logBody) > 1024 {
			logBody = logBody[:1024] + "..."
		}
		log.Printf("CoinMarketCap: Помилка статусу %d від %s. Тіло: %s", resp.StatusCode, reqURL, logBody)

		if json.Unmarshal(bodyBytes, &errorResponse) == nil && errorResponse.Status.ErrorCode != 0 {
			return nil, fmt.Errorf("API CoinMarketCap (%s) повернуло помилку: %s (код %d)", endpointPath, errorResponse.Status.ErrorMessage, errorResponse.Status.ErrorCode)
		}
		return nil, fmt.Errorf("статус %d від CoinMarketCap API (%s)", resp.StatusCode, endpointPath)
	}
	return bodyBytes, nil
}

func GetTopMarketCapCoinsCMC(apiKey string, limit int) ([]UnifiedCoinInfoCMC, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API ключ CoinMarketCap не надано")
	}
	if limit <= 0 {
		log.Printf("CoinMarketCap: Неправильний ліміт %d для GetTopMarketCapCoinsCMC, встановлено на 20", limit)
		limit = 20
	}

	params := url.Values{}
	params.Add("start", "1")
	params.Add("limit", strconv.Itoa(limit))
	params.Add("convert", "USD")

	log.Printf("CoinMarketCap: Запит топ-%d монет.", limit)
	bodyBytes, err := makeCMCRequest(apiKey, listingsLatestPath, params)
	if err != nil {
		return nil, fmt.Errorf("запит до CMC /listings/latest не вдався: %w", err)
	}

	var cmcResponse CMCListingsLatestResponse
	if err := json.Unmarshal(bodyBytes, &cmcResponse); err != nil {
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		log.Printf("CoinMarketCap: Не вдалося розпарсити JSON з /listings/latest. Тіло: %s", bodyStr)
		return nil, fmt.Errorf("декодування відповіді /listings/latest від CMC: %w", err)
	}

	if cmcResponse.Status.ErrorCode != 0 {
		log.Printf("CoinMarketCap: API /listings/latest повернуло логічну помилку: %s (код %d), Credits: %d", cmcResponse.Status.ErrorMessage, cmcResponse.Status.ErrorCode, cmcResponse.Status.CreditCount)
		return nil, fmt.Errorf("API CMC /listings/latest помилка: %s (код %d)", cmcResponse.Status.ErrorMessage, cmcResponse.Status.ErrorCode)
	}
	
	if len(cmcResponse.Data) == 0 && cmcResponse.Status.ErrorCode == 0 {
		log.Printf("CoinMarketCap: Отримано 0 монет з /listings/latest для ліміту %d. Credits: %d", limit, cmcResponse.Status.CreditCount)
	}

	var coins []UnifiedCoinInfoCMC
	for _, coinData := range cmcResponse.Data {
		coins = append(coins, UnifiedCoinInfoCMC{
			ID:     strconv.Itoa(coinData.ID),
			Symbol: strings.ToUpper(coinData.Symbol),
			Name:   coinData.Name,
		})
	}
	log.Printf("CoinMarketCap: Успішно отримано %d монет (запит був на %d). Credits використано: %d", len(coins), limit, cmcResponse.Status.CreditCount)
	return coins, nil
}

// GetCoinTickersCMC отримує тікери для конкретної монети з CoinMarketCap
// coinIdentifier може бути ID монети (числовим) або її символом.
func GetCoinTickersCMC(apiKey string, coinIdentifier string) ([]UnifiedTickerInfoCMC, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API ключ CoinMarketCap не надано")
	}
	if coinIdentifier == "" {
		return nil, fmt.Errorf("ідентифікатор монети для GetCoinTickersCMC не надано")
	}

	params := url.Values{}
	// CoinMarketCap дозволяє шукати за ID, slug або symbol.
	// Якщо coinIdentifier - це число, вважаємо, що це ID. Інакше - symbol.
	// У нашому UnifiedCoinInfoCMC.ID зберігається числовий ID як рядок.
	if _, err := strconv.Atoi(coinIdentifier); err == nil {
		params.Add("id", coinIdentifier)
	} else {
		// Якщо не число, то це може бути slug або symbol.
		// Ендпоінт /market-pairs/latest краще працює з ID або slug.
		// Якщо це символ, можливо, знадобиться попередньо отримати ID/slug.
		// Поки що припустимо, що coinIdentifier - це ID (як рядок) або slug,
		// або символ, який API зможе розпізнати. Для надійності краще передавати ID.
		params.Add("slug", coinIdentifier) // Спробуємо slug, якщо це не ID
		// Або params.Add("symbol", strings.ToUpper(coinIdentifier))
	}
	params.Add("convert", "USD")
	// ліміт тікерів, за замовчуванням 100, макс 5000 на деяких планах.
	// Для спредів зазвичай достатньо ~20-50 топ-тікерів за обсягом, але API не сортує їх так.
	params.Add("limit", "100") 
	// Можна додати &aux=market_cap_by_total_supply,effective_liquidity для отримання дод. даних
	// params.Add("aux", "effective_liquidity,market_score") // Для отримання score

	log.Printf("CoinMarketCap: Запит тікерів для монети '%s'.", coinIdentifier)
	bodyBytes, err := makeCMCRequest(apiKey, marketPairsPath, params)
	if err != nil {
		return nil, fmt.Errorf("запит до CMC /market-pairs/latest не вдався: %w", err)
	}

	var cmcResponse CMCMarketPairsResponse
	if err := json.Unmarshal(bodyBytes, &cmcResponse); err != nil {
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		log.Printf("CoinMarketCap: Не вдалося розпарсити JSON з /market-pairs/latest для '%s'. Тіло: %s", coinIdentifier, bodyStr)
		return nil, fmt.Errorf("декодування відповіді /market-pairs/latest від CMC для '%s': %w", coinIdentifier, err)
	}

	if cmcResponse.Status.ErrorCode != 0 {
		log.Printf("CoinMarketCap: API /market-pairs/latest для '%s' повернуло логічну помилку: %s (код %d), Credits: %d", coinIdentifier, cmcResponse.Status.ErrorMessage, cmcResponse.Status.ErrorCode, cmcResponse.Status.CreditCount)
		return nil, fmt.Errorf("API CMC /market-pairs/latest помилка для '%s': %s (код %d)", coinIdentifier, cmcResponse.Status.ErrorMessage, cmcResponse.Status.ErrorCode)
	}

	if len(cmcResponse.Data.MarketPairs) == 0 && cmcResponse.Status.ErrorCode == 0 {
        log.Printf("CoinMarketCap: Отримано 0 тікерів з /market-pairs/latest для '%s' (CoinID: %d, CoinName: %s). Credits: %d", coinIdentifier, cmcResponse.Data.ID, cmcResponse.Data.Name, cmcResponse.Status.CreditCount)
    }

	var tickers []UnifiedTickerInfoCMC
	for _, pair := range cmcResponse.Data.MarketPairs {
		// Нас цікавлять тільки пари до USDT
		if strings.ToUpper(pair.QuoteSymbol) != "USDT" {
			continue
		}

		priceData, ok := pair.Quote["USD"]
		if !ok || priceData.Price <= 0 { // Перевіряємо, чи є дані про ціну і чи вона позитивна
			// log.Printf("CoinMarketCap: Пропуск пари %s/%s на біржі %s через відсутність або нульову ціну в USD.", pair.BaseSymbol, pair.QuoteSymbol, pair.Exchange.Name)
			continue
		}
		
		// Важливо: BaseSymbol та QuoteSymbol можуть бути надійнішими, ніж парсинг MarketPair
		baseCurrency := strings.ToUpper(pair.BaseSymbol)
		quoteCurrency := strings.ToUpper(pair.QuoteSymbol)

		tickers = append(tickers, UnifiedTickerInfoCMC{
			ExchangeName:       pair.Exchange.Name,
			ExchangeIdentifier: pair.Exchange.Slug, // Використовуємо slug як біржовий ідентифікатор
			BaseCurrency:       baseCurrency,
			QuoteCurrency:      quoteCurrency,
			PriceUSD:           priceData.Price,
			Volume24hUSD:       priceData.Volume24h,
			TrustScore:         "N/A", // Як і домовилися, поки "N/A"
			TradeURL:           "",    // CMC зазвичай не надає прямих торгових URL
		})
	}
	log.Printf("CoinMarketCap: Успішно отримано %d USDT тікерів для '%s' (CoinID: %d, Name: %s). Всього знайдено пар: %d. Credits використано: %d",
		len(tickers), coinIdentifier, cmcResponse.Data.ID, cmcResponse.Data.Name, cmcResponse.Data.NumMarketPairs, cmcResponse.Status.CreditCount)
	return tickers, nil
}
