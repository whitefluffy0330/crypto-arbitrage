package config

import (
	"log"
	"os"
	"strconv"
)

// Config зберігає всі конфігураційні параметри застосунку
type Config struct {
	BotToken         string
	SpreadsheetID    string
	ChatID           int64 

	// Конфігурація Google Sheets
	SheetNameReport         string 
	SheetRangeReport        string 
	SheetNameUserGoals      string 
	SheetRangeUserGoals     string 
	SheetNameWorkLog        string 
	SheetRangeWorkLogDates  string 
	SheetRangeWorkLogFull   string 
	SheetNameInvestments    string 

	// Конфігурація Webhook та сервера
	WebhookBaseURL    string 
	WebhookPath       string 
	WebhookListenAddr string // Адреса, на якій слухає Go-бот (напр., "localhost:8080")
	WebhookCertPath   string // Шлях до сертифіката для SetWebhook (зазвичай порожньо, якщо Nginx обробляє TLS)
	TLSCertPath       string // Шлях до fullchain.pem (для Nginx)
	TLSKeyPath        string // Шлях до privkey.pem (для Nginx)
}

// LoadEnv завантажує конфігурацію зі змінних середовища
func LoadEnv() Config {
	botToken := os.Getenv("TELEGRAM_TOKEN")
	spreadsheetID := os.Getenv("SPREADSHEET_ID")
	webhookBaseURL := os.Getenv("WEBHOOK_BASE_URL") 
	webhookPath := os.Getenv("WEBHOOK_PATH")       
	
	// Ці шляхи тепер для Nginx, бот їх напряму не використовує для запуску сервера
	tlsCertPath := os.Getenv("TLS_CERT_PATH")      
	tlsKeyPath := os.Getenv("TLS_KEY_PATH")        

	// Перевірка критично важливих змінних
	if botToken == "" { log.Fatal("Критична помилка: Змінна TELEGRAM_TOKEN не встановлена.") }
	if spreadsheetID == "" { log.Fatal("Критична помилка: Змінна SPREADSHEET_ID не встановлена.") }
	if webhookBaseURL == "" { log.Fatal("Критична помилка: Змінна WEBHOOK_BASE_URL не встановлена.") }
	if webhookPath == "" { log.Fatal("Критична помилка: Змінна WEBHOOK_PATH не встановлена.") }
	// Ці шляхи важливі для налаштування Nginx, але бот може запуститися і без них, якщо Nginx налаштований окремо
	if tlsCertPath == "" { log.Println("ПОПЕРЕДЖЕННЯ: Змінна середовища TLS_CERT_PATH не встановлена. Nginx має бути налаштований з правильними сертифікатами.") }
	if tlsKeyPath == "" { log.Println("ПОПЕРЕДЖЕННЯ: Змінна середовища TLS_KEY_PATH не встановлена. Nginx має бути налаштований з правильними сертифікатами.") }


	var chatID int64 
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if chatIDStr == "" { log.Printf("ПОПЕРЕДЖЕННЯ: Змінна TELEGRAM_CHAT_ID не встановлена. ChatID буде 0.") } else {
		parsedChatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil { log.Printf("ПОМИЛКА: Не розпарсено TELEGRAM_CHAT_ID '%s': %v. ChatID=0.", chatIDStr, err) } else { chatID = parsedChatID }
	}

	cfg := Config{
		BotToken:         botToken,
		SpreadsheetID:    spreadsheetID,
		ChatID:           chatID,

		SheetNameReport:         getEnv("SHEET_NAME_REPORT", "Звіт"),
		SheetRangeReport:        getEnv("SHEET_RANGE_REPORT", "A2:E2"), 
		SheetNameUserGoals:      getEnv("SHEET_NAME_USER_GOALS", "МоїЦілі"),
		SheetRangeUserGoals:     getEnv("SHEET_RANGE_USER_GOALS", "A:H"), 
		SheetNameWorkLog:        getEnv("SHEET_NAME_WORK_LOG", "РобочийГрафік"),
		SheetRangeWorkLogDates:  getEnv("SHEET_RANGE_WORK_LOG_DATES", "A:B"), 
		SheetRangeWorkLogFull:   getEnv("SHEET_RANGE_WORK_LOG_FULL", "A:E"),  
		SheetNameInvestments:    getEnv("SHEET_NAME_INVESTMENTS", "Інвестиції"),

		WebhookBaseURL:    webhookBaseURL,
		WebhookPath:       webhookPath,
		WebhookListenAddr: getEnv("WEBHOOK_LISTEN_ADDR", "localhost:8080"), // Змінено значення за замовчуванням
		WebhookCertPath:   os.Getenv("WEBHOOK_CERT_PATH"), // Залишаємо, Telegram може його перевіряти       
		TLSCertPath:       tlsCertPath, 
		TLSKeyPath:        tlsKeyPath,  
	}

	log.Printf("Конфігурацію завантажено: SpreadsheetID=%s, ReportSheet='%s!%s', GoalsSheet='%s', WorkLogSheet='%s', InvestmentsSheet='%s', Webhook=%s%s, BotListenAddr=%s, AdminChatID=%d",
		cfg.SpreadsheetID, cfg.SheetNameReport, cfg.SheetRangeReport,
		cfg.SheetNameUserGoals, cfg.SheetNameWorkLog, cfg.SheetNameInvestments, 
		cfg.WebhookBaseURL, cfg.WebhookPath, cfg.WebhookListenAddr, cfg.ChatID)

	return cfg
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	log.Printf("ПОПЕРЕДЖЕННЯ: Змінна середовища %s не встановлена або порожня, використовується '%s'", key, fallback)
	return fallback
}
