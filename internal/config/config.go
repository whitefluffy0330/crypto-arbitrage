package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time" // Потрібен для time.ParseDuration
)

// Config зберігає всі конфігураційні параметри
type Config struct {
	BotToken                  string
	SpreadsheetID             string
	GoogleAppCredentialsJSON  string // Шлях до credentials.json
	WebhookBaseURL            string
	WebhookPath               string
	WebhookListenAddr         string // Адреса, яку слухає бот, наприклад, "localhost:8080"
	WebhookCertPath           string // Шлях до SSL сертифіката (якщо використовується самопідписаний, або якщо бот сам обробляє HTTPS)
	WebhookKeyPath            string // Шлях до SSL ключа
	AdminChatID               int64  // Опціональний ChatID для адмінських повідомлень
	SheetNameUserGoals        string `env:"SHEET_NAME_USER_GOALS,default=МоїЦілі"`
	SheetNameWorkLog          string `env:"SHEET_NAME_WORK_LOG,default=РобочийГрафік"`
	SheetNameReport           string `env:"SHEET_NAME_REPORT,default=Звіт"`
	SheetRangeReport          string `env:"SHEET_RANGE_REPORT,default=A2:E2"` // Діапазон для читання даних звіту
	SheetNameInvestments      string `env:"SHEET_NAME_INVESTMENTS,default=Інвестиції"`
	HttpTimeoutSeconds        int    `env:"HTTP_TIMEOUT_SECONDS,default=10"`
	MaxConcurrentExchangeReqs int    `env:"MAX_CONCURRENT_EXCHANGE_REQS,default=5"` // Загальне обмеження
	SpreadCoinCount           int    `env:"SPREAD_COIN_COUNT,default=50"`
	SpreadMinPercentage       float64  `env:"SPREAD_MIN_PERCENTAGE,default=2.0"`
	SpreadUserExchanges       []string `env:"SPREAD_USER_EXCHANGES,default=binance,bybit,okx,mexc,bitget"`
	SpreadMinTrustScore       string   `env:"SPREAD_MIN_TRUST_SCORE,default=green"`
}

// getEnv читає змінну середовища або повертає значення за замовчуванням
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// LoadEnv завантажує конфігурацію зі змінних середовища
func LoadEnv() Config {
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
		log.Fatal("Критична помилка: GOOGLE_APPLICATION_CREDENTIALS не встановлено!")
	}

	adminChatIDStr := getEnv("TELEGRAM_CHAT_ID", "0")
	adminChatID, err := strconv.ParseInt(adminChatIDStr, 10, 64)
	if err != nil {
		log.Printf("ПОПЕРЕДЖЕННЯ: Неправильний формат TELEGRAM_CHAT_ID: %v. ChatID буде 0.", err)
		adminChatID = 0
	} else if adminChatID == 0 {
		log.Println("ПОПЕРЕДЖЕННЯ: Змінна TELEGRAM_CHAT_ID не встановлена або 0. Адмінські повідомлення не надсилатимуться на конкретний ChatID.")
	}

	httpTimeoutSecondsStr := getEnv("HTTP_TIMEOUT_SECONDS", "10")
	httpTimeoutSeconds, err := strconv.Atoi(httpTimeoutSecondsStr)
	if err != nil || httpTimeoutSeconds <= 0 {
		log.Printf("Помилка парсингу HTTP_TIMEOUT_SECONDS: %v, використовується значення за замовчуванням 10", err)
		httpTimeoutSeconds = 10
	}

	maxConcurrentExchangeReqsStr := getEnv("MAX_CONCURRENT_EXCHANGE_REQS", "5")
	maxConcurrentExchangeReqs, err := strconv.Atoi(maxConcurrentExchangeReqsStr)
	if err != nil || maxConcurrentExchangeReqs <= 0 {
		log.Printf("Помилка парсингу MAX_CONCURRENT_EXCHANGE_REQS: %v, використовується значення за замовчуванням 5", err)
		maxConcurrentExchangeReqs = 5
	}
	
	spreadCoinCountStr := getEnv("SPREAD_COIN_COUNT", "50")
	spreadCoinCount, err := strconv.Atoi(spreadCoinCountStr)
	if err != nil {
		log.Printf("Помилка парсингу SPREAD_COIN_COUNT: %v, використовується значення за замовчуванням 50", err)
		spreadCoinCount = 50
	}

	spreadMinPercentageStr := getEnv("SPREAD_MIN_PERCENTAGE", "2.0")
	spreadMinPercentage, err := strconv.ParseFloat(spreadMinPercentageStr, 64)
	if err != nil {
		log.Printf("Помилка парсингу SPREAD_MIN_PERCENTAGE: %v, використовується значення за замовчуванням 2.0", err)
		spreadMinPercentage = 2.0
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
	
	spreadMinTrustScore := getEnv("SPREAD_MIN_TRUST_SCORE", "green")


	cfg := Config{
		BotToken:                  botToken,
		SpreadsheetID:             spreadsheetID,
		GoogleAppCredentialsJSON:  googleAppCredentialsJSON,
		WebhookBaseURL:            getEnv("WEBHOOK_BASE_URL", "https://yourdomain.com"), // Замініть на ваш домен або отримайте з env
		WebhookPath:               getEnv("WEBHOOK_PATH", "/your-secret-webhook-path"),   // Замініть на ваш шлях
		WebhookListenAddr:         getEnv("WEBHOOK_LISTEN_ADDR", "localhost:8080"),
		WebhookCertPath:           getEnv("TLS_CERT_PATH", ""), // Залиште порожнім, якщо Nginx обробляє TLS
		WebhookKeyPath:            getEnv("TLS_KEY_PATH", ""),  // Залиште порожнім, якщо Nginx обробляє TLS
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
	}
	log.Printf("Конфігурацію завантажено: SpreadsheetID=%s, ReportSheet='%s', GoalsSheet='%s', WorkLogSheet='%s', InvestmentsSheet='%s', Webhook=%s%s, BotListenAddr=%s, AdminChatID=%d, SpreadCoinCount=%d, SpreadMinPercentage=%.2f, SpreadUserExchanges=%v, SpreadMinTrustScore='%s'",
		cfg.SpreadsheetID, cfg.SheetNameReport, cfg.SheetNameUserGoals, cfg.SheetNameWorkLog, cfg.SheetNameInvestments, cfg.WebhookBaseURL, cfg.WebhookPath, cfg.WebhookListenAddr, cfg.AdminChatID, cfg.SpreadCoinCount, cfg.SpreadMinPercentage, cfg.SpreadUserExchanges, cfg.SpreadMinTrustScore)
	return cfg
}
