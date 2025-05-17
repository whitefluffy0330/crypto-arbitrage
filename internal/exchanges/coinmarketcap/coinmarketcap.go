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
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/config" // Може знадобитися для ліміту
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
	TrustScore         string // Заповнюватиметься як "N/A" або аналогічно
	TradeURL           string // Залишатиметься порожнім, якщо CMC не надає
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

	client := http.Client{Timeout: 20 * time.Second} // Збільшено таймаут
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
		// Спробуємо розпарсити помилку, якщо вона є в JSON форматі
		var errorResponse struct {
			Status struct {
				ErrorCode    int    `json:"error_code"`
				ErrorMessage string `json:"error_message"`
			} `json:"status"`
		}
		// Логуємо сире тіло помилки перед спробою парсингу
		logBody := string(bodyBytes)
		if len(logBody) > 1024 { // Обмежимо довжину логу
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

// GetTopMarketCapCoinsCMC отримує топ-N монет з CoinMarketCap
// Важливо: 'limit' тут визначає, скільки монет запитувати у API.
// Якщо ви хочете використовувати cfg.SpreadCoinCount, його потрібно передати сюди.
func GetTopMarketCapCoinsCMC(apiKey string, limit int) ([]UnifiedCoinInfoCMC, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API ключ CoinMarketCap не надано")
	}
	if limit <= 0 {
		log.Printf("CoinMarketCap: Неправильний ліміт %d для GetTopMarketCapCoinsCMC, встановлено на 20", limit)
		limit = 20 // Безпечне значення за замовчуванням, якщо передано невірне
	}

	params := url.Values{}
	params.Add("start", "1")
	params.Add("limit", strconv.Itoa(limit))
	params.Add("convert", "USD")
	// params.Add("sort", "market_cap") // За замовчуванням сортування за ринковою капіталізацією
	// params.Add("sort_dir", "desc")

	log.Printf("CoinMarketCap: Запит топ-%d монет.", limit)
	bodyBytes, err := makeCMCRequest(apiKey, listingsLatestPath, params)
	if err != nil {
		// помилка вже детально логується в makeCMCRequest або при HTTP помилці
		return nil, fmt.Errorf("запит до CMC /listings/latest не вдався: %w", err)
	}

	var cmcResponse CMCListingsLatestResponse
	if err := json.Unmarshal(bodyBytes, &cmcResponse); err != nil {
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		// Це логування допоможе побачити, що саме не так з JSON
		log.Printf("CoinMarketCap: Не вдалося розпарсити JSON з /listings/latest. Тіло: %s", bodyStr)
		return nil, fmt.Errorf("декодування відповіді /listings/latest від CMC: %w", err)
	}

	// Перевірка на логічні помилки API після успішного парсингу
	if cmcResponse.Status.ErrorCode != 0 {
		log.Printf("CoinMarketCap: API /listings/latest повернуло логічну помилку: %s (код %d), Credits: %d", cmcResponse.Status.ErrorMessage, cmcResponse.Status.ErrorCode, cmcResponse.Status.CreditCount)
		return nil, fmt.Errorf("API CMC /listings/latest помилка: %s (код %d)", cmcResponse.Status.ErrorMessage, cmcResponse.Status.ErrorCode)
	}

	if len(cmcResponse.Data) == 0 {
		log.Printf("CoinMarketCap: Отримано 0 монет з /listings/latest для ліміту %d. Перевірте API ключ або параметри запиту. Credits: %d", limit, cmcResponse.Status.CreditCount)
        // Не повертаємо помилку, а порожній зріз, якщо API відпрацювало коректно, але даних немає
	}


	var coins []UnifiedCoinInfoCMC
	for _, coinData := range cmcResponse.Data {
		coins = append(coins, UnifiedCoinInfoCMC{
			ID:     strconv.Itoa(coinData.ID), // Використовуємо числовий ID як рядок
			Symbol: strings.ToUpper(coinData.Symbol),
			Name:   coinData.Name,
		})
	}
	log.Printf("CoinMarketCap: Успішно отримано %d монет (запит був на %d). Credits використано: %d", len(coins), limit, cmcResponse.Status.CreditCount)
	return coins, nil
}

// Наступною буде функція GetCoinTickersCMC
// ... (решта коду, включаючи GetCoinTickersCMC, буде додана пізніше)
