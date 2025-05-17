package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	// "time" // Не використовується тут напряму
)

// Config зберігає всі конфігураційні параметри застосунку
type Config struct {
	BotToken                 string
	SpreadsheetID            string
	GoogleAppCredentialsJSON string 
	
	WebhookBaseURL           string 
	WebhookPath              string 
	WebhookListenAddr        string 
	WebhookCertPath          string 
	TLSCertPath              string 
	TLSKeyPath               string 

	AdminChatID              int64  
	
	SheetNameUserGoals       string 
	SheetNameWorkLog         string 
	SheetNameReport          string 
	SheetRangeReport         string 
	SheetNameInvestments     string 
	
	HttpTimeoutSeconds        int    
	MaxConcurrentExchangeReqs int    

	SpreadCoinCount       int      
	SpreadMinPercentage   float64  
	SpreadUserExchanges   []string 
	SpreadMinTrustScore   string   
	CoinMarketCapAPIKey   string   // ДОДАНО: API ключ для CoinMarketCap
}

// getEnv читає змінну середовища або повертає значення за замовчуванням
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

// LoadEnv завантажує конфігурацію зі змінних середовища
func LoadEnv() Config {
	// ... (існуючий код завантаження TELEGRAM_TOKEN, SPREADSHEET_ID, GOOGLE_APPLICATION_CREDENTIALS) ...
	botToken := getEnv("TELEGRAM_TOKEN", "")
	if botToken == "" {
		log.Fatal("Критична помилка: TELEGRAM_TOKEN не встановлено!")
	}
	spreadsheetID := getEnv("SPREADSHEET_ID", "")
	if spreadsheetID == "" {
		log.Fatal("Критична помилка: SPREADSHEET_ID не встановлено!")
	}
	googleAppCredentialsJSON := getEnv("GOOGLE_APPLICATION_CREDENTIALS", "")
	if googleAppCredentialsJSON == "" {
		log.Println("ПОПЕРЕДЖЕННЯ: GOOGLE_APPLICATION_CREDENTIALS не встановлено.")
	}

	adminChatIDStr := getEnv("TELEGRAM_CHAT_ID", "0") 
	adminChatID, err := strconv.ParseInt(adminChatIDStr, 10, 64)
	if err != nil {
		log.Printf("ПОПЕРЕДЖЕННЯ: Неправильний формат TELEGRAM_CHAT_ID ('%s'): %v. ChatID буде 0.", adminChatIDStr, err)
		adminChatID = 0
	} else if adminChatID == 0 && adminChatIDStr != "0"{ 
		log.Printf("ПОПЕРЕДЖЕННЯ: TELEGRAM_CHAT_ID розпарсено як 0 з '%s'.", adminChatIDStr)
	} else if adminChatIDStr == "" { 
         log.Println("ПОПЕРЕДЖЕННЯ: Змінна TELEGRAM_CHAT_ID не встановлена. ChatID буде 0.")
    }

	httpTimeoutSecondsStr := getEnv("HTTP_TIMEOUT_SECONDS", "15")
	httpTimeoutSeconds, err := strconv.Atoi(httpTimeoutSecondsStr)
	if err != nil || httpTimeoutSeconds <= 0 {
		log.Printf("Помилка парсингу HTTP_TIMEOUT_SECONDS: %v, використовується значення за замовчуванням 15", err)
		httpTimeoutSeconds = 15
	}

	maxConcurrentExchangeReqsStr := getEnv("MAX_CONCURRENT_EXCHANGE_REQS", "5")
	maxConcurrentExchangeReqs, err := strconv.Atoi(maxConcurrentExchangeReqsStr)
	if err != nil || maxConcurrentExchangeReqs <= 0 {
		log.Printf("Помилка парсингу MAX_CONCURRENT_EXCHANGE_REQS: %v, використовується значення за замовчуванням 5", err)
		maxConcurrentExchangeReqs = 5
	}
	
	spreadCoinCountStr := getEnv("SPREAD_COIN_COUNT", "10") // Зменшено дефолт
	spreadCoinCount, err := strconv.Atoi(spreadCoinCountStr)
	if err != nil || spreadCoinCount <= 0 {
		log.Printf("Помилка парсингу SPREAD_COIN_COUNT ('%s'): %v, використовується значення за замовчуванням 10", spreadCoinCountStr, err)
		spreadCoinCount = 10
	}

	spreadMinPercentageStr := getEnv("SPREAD_MIN_PERCENTAGE", "0.5") // Зменшено дефолт для тестів
	spreadMinPercentage, err := strconv.ParseFloat(spreadMinPercentageStr, 64)
	if err != nil || spreadMinPercentage < 0 {
		log.Printf("Помилка парсингу SPREAD_MIN_PERCENTAGE ('%s'): %v, використовується значення за замовчуванням 0.5", spreadMinPercentageStr, err)
		spreadMinPercentage = 0.5
	}

	spreadUserExchangesStr := getEnv("SPREAD_USER_EXCHANGES", "binance,bybit,okx,mexc,bitget") 
	var spreadUserExchanges []string
	if spreadUserExchangesStr != "" {
		rawExchanges := strings.Split(spreadUserExchangesStr, ",")
		for _, ex := range rawExchanges {
			trimmedEx := strings.ToLower(strings.TrimSpace(ex))
			if trimmedEx != "" {
				spreadUserExchanges = append(spreadUserExchanges, trimmedEx)
			}
		}
	}
	if len(spreadUserExchanges) == 0 { 
		log.Println("ПОПЕРЕДЖЕННЯ: SPREAD_USER_EXCHANGES не встановлено або порожній. Використовуються біржі за замовчуванням: binance, bybit.")
		spreadUserExchanges = []string{"binance", "bybit"}
	}
	
	spreadMinTrustScore := getEnv("SPREAD_MIN_TRUST_SCORE", "green")
	coinMarketCapAPIKey := getEnv("COINMARKETCAP_API_KEY", "") // ДОДАНО завантаження ключа
	if coinMarketCapAPIKey == "" {
		log.Println("ПОПЕРЕДЖЕННЯ: COINMARKETCAP_API_KEY не встановлено. Функція спредів через CoinMarketCap буде недоступна.")
	}

	cfg := Config{
		BotToken:                  botToken,
		SpreadsheetID:             spreadsheetID,
		GoogleAppCredentialsJSON:  googleAppCredentialsJSON,
		WebhookBaseURL:            getEnv("WEBHOOK_BASE_URL", ""), 
		WebhookPath:               getEnv("WEBHOOK_PATH", ""),   
		WebhookListenAddr:         getEnv("WEBHOOK_LISTEN_ADDR", "localhost:8080"),
		WebhookCertPath:           getEnv("TLS_CERT_PATH", ""),      
		TLSCertPath:               getEnv("TLS_CERT_PATH_NGINX", ""),  
		TLSKeyPath:                getEnv("TLS_KEY_PATH_NGINX", ""),  
		AdminChatID:               adminChatID,
		SheetNameUserGoals:        getEnv("SHEET_NAME_USER_GOALS", "МоїЦілі"),
		SheetNameWorkLog:          getEnv("SHEET_NAME_WORK_LOG", "РобочийГрафік"),
		SheetNameReport:           getEnv("SHEET_NAME_REPORT", "Звіт"),
		SheetRangeReport:          getEnv("SHEET_RANGE_REPORT", "A2:E2"),
		SheetNameInvestments:      getEnv("SHEET_NAME_INVESTMENTS", "Інвестиції"),
		HttpTimeoutSeconds:        httpTimeoutSeconds,
		MaxConcurrentExchangeReqs: maxConcurrentExchangeReqs,
		SpreadCoinCount:           spreadCoinCount,
		SpreadMinPercentage:       spreadMinPercentage,
		SpreadUserExchanges:       spreadUserExchanges,
		SpreadMinTrustScore:       strings.ToLower(spreadMinTrustScore),
		CoinMarketCapAPIKey:       coinMarketCapAPIKey, // ДОДАНО
	}
	log.Printf("Конфігурацію завантажено: ... SpreadCoinCount=%d, SpreadMinPercentage=%.2f, SpreadUserExchanges=%v, SpreadMinTrustScore='%s', HasCoinMarketCapKey: %t",
		cfg.SpreadCoinCount, cfg.SpreadMinPercentage, cfg.SpreadUserExchanges, cfg.SpreadMinTrustScore, cfg.CoinMarketCapAPIKey != "")
	return cfg
}
