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
	GoogleAppCredentialsJSON string // Шлях до credentials.json
	
	WebhookBaseURL           string 
	WebhookPath              string 
	WebhookListenAddr        string 
	WebhookCertPath          string // Для SetWebhook, якщо бот сам обробляє TLS
	TLSCertPath              string // Для Nginx
	TLSKeyPath               string // Для Nginx

	AdminChatID              int64  
	
	SheetNameUserGoals       string 
	SheetNameWorkLog         string 
	SheetNameReport          string 
	SheetRangeReport         string 
	SheetNameInvestments     string 
	
	HttpTimeoutSeconds        int    
	MaxConcurrentExchangeReqs int    

	// Нові параметри для функції спредів
	SpreadCoinCount       int      
	SpreadMinPercentage   float64  
	SpreadUserExchanges   []string 
	SpreadMinTrustScore   string   
}

// getEnv читає змінну середовища або повертає значення за замовчуванням
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" { // Додано перевірку на порожній рядок
		return value
	}
	// Не логуємо тут, щоб не спамити, якщо змінна опціональна і має дефолт
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
		// Це може бути опціонально, якщо сервісний акаунт налаштовано інакше (наприклад, на VM)
		log.Println("ПОПЕРЕДЖЕННЯ: GOOGLE_APPLICATION_CREDENTIALS не встановлено. Авторизація до Google Sheets може не спрацювати, якщо не налаштовано іншим способом.")
	}

	adminChatIDStr := getEnv("TELEGRAM_CHAT_ID", "0") // За замовчуванням 0, якщо не встановлено
	adminChatID, err := strconv.ParseInt(adminChatIDStr, 10, 64)
	if err != nil {
		log.Printf("ПОПЕРЕДЖЕННЯ: Неправильний формат TELEGRAM_CHAT_ID ('%s'): %v. ChatID буде 0.", adminChatIDStr, err)
		adminChatID = 0
	} else if adminChatID == 0 && adminChatIDStr != "0"{ // Якщо було введено не "0", але розпарсилось як 0
		log.Printf("ПОПЕРЕДЖЕННЯ: TELEGRAM_CHAT_ID розпарсено як 0 з '%s'. Адмінські повідомлення не надсилатимуться на конкретний ChatID.", adminChatIDStr)
	} else if adminChatIDStr == "" { // Якщо змінна середовища порожня
         log.Println("ПОПЕРЕДЖЕННЯ: Змінна TELEGRAM_CHAT_ID не встановлена. ChatID буде 0.")
    }


	httpTimeoutSecondsStr := getEnv("HTTP_TIMEOUT_SECONDS", "15") // Збільшено дефолт
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
	
	// Завантаження параметрів для спредів
	spreadCoinCountStr := getEnv("SPREAD_COIN_COUNT", "20") // Зменшено дефолт для початку
	spreadCoinCount, err := strconv.Atoi(spreadCoinCountStr)
	if err != nil || spreadCoinCount <= 0 {
		log.Printf("Помилка парсингу SPREAD_COIN_COUNT ('%s'): %v, використовується значення за замовчуванням 20", spreadCoinCountStr, err)
		spreadCoinCount = 20
	}

	spreadMinPercentageStr := getEnv("SPREAD_MIN_PERCENTAGE", "2.0")
	spreadMinPercentage, err := strconv.ParseFloat(spreadMinPercentageStr, 64)
	if err != nil || spreadMinPercentage < 0 {
		log.Printf("Помилка парсингу SPREAD_MIN_PERCENTAGE ('%s'): %v, використовується значення за замовчуванням 2.0", spreadMinPercentageStr, err)
		spreadMinPercentage = 2.0
	}

	// Важливо: SpreadUserExchanges тепер правильно обробляє порожній рядок або відсутність змінної
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
	if len(spreadUserExchanges) == 0 { // Якщо після всіх маніпуляцій список порожній, встановлюємо дефолт
		log.Println("ПОПЕРЕДЖЕННЯ: SPREAD_USER_EXCHANGES не встановлено або порожній. Використовуються біржі за замовчуванням: binance, bybit.")
		spreadUserExchanges = []string{"binance", "bybit"}
	}
	
	spreadMinTrustScore := getEnv("SPREAD_MIN_TRUST_SCORE", "green")


	cfg := Config{
		BotToken:                  botToken,
		SpreadsheetID:             spreadsheetID,
		GoogleAppCredentialsJSON:  googleAppCredentialsJSON,
		WebhookBaseURL:            getEnv("WEBHOOK_BASE_URL", ""), 
		WebhookPath:               getEnv("WEBHOOK_PATH", ""),   
		WebhookListenAddr:         getEnv("WEBHOOK_LISTEN_ADDR", "localhost:8080"),
		WebhookCertPath:           getEnv("TLS_CERT_PATH", ""), // Залишаємо можливість для бота обробляти TLS
		TLSCertPath:               getEnv("TLS_CERT_PATH_NGINX", ""), // Окремо для Nginx, якщо потрібно в конфігу      
		TLSKeyPath:                getEnv("TLS_KEY_PATH_NGINX", ""),  // Окремо для Nginx
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
	log.Printf("Конфігурацію завантажено: ... SpreadCoinCount=%d, SpreadMinPercentage=%.2f, SpreadUserExchanges=%v, SpreadMinTrustScore='%s'",
		cfg.SpreadCoinCount, cfg.SpreadMinPercentage, cfg.SpreadUserExchanges, cfg.SpreadMinTrustScore) // Додано до логування
	return cfg
}
