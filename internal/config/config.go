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
	SheetNameInvestments    string // <<< НОВЕ ПОЛЕ: Назва аркуша для інвестицій

	// Конфігурація Webhook та TLS
	WebhookBaseURL    string 
	WebhookPath       string 
	WebhookListenAddr string 
	WebhookCertPath   string 
	TLSCertPath       string 
	TLSKeyPath        string 
}

// LoadEnv завантажує конфігурацію зі змінних середовища
func LoadEnv() Config {
	botToken := os.Getenv("TELEGRAM_TOKEN")
	spreadsheetID := os.Getenv("SPREADSHEET_ID")
	webhookBaseURL := os.Getenv("WEBHOOK_BASE_URL") 
	webhookPath := os.Getenv("WEBHOOK_PATH")       
	tlsCertPath := os.Getenv("TLS_CERT_PATH")      
	tlsKeyPath := os.Getenv("TLS_KEY_PATH")        

	if botToken == "" { log.Fatal("Крит. помилка: Змінна TELEGRAM_TOKEN не встановлена.") }
	if spreadsheetID == "" { log.Fatal("Крит. помилка: Змінна SPREADSHEET_ID не встановлена.") }
	if webhookBaseURL == "" { log.Fatal("Крит. помилка: Змінна WEBHOOK_BASE_URL не встановлена.") }
	if webhookPath == "" { log.Fatal("Крит. помилка: Змінна WEBHOOK_PATH не встановлена.") }
	if tlsCertPath == "" { log.Fatal("Крит. помилка: Змінна TLS_CERT_PATH не встановлена.") }
	if tlsKeyPath == "" { log.Fatal("Крит. помилка: Змінна TLS_KEY_PATH не встановлена.") }

	var chatID int64 
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if chatIDStr == "" { log.Printf("ПОПЕРЕДЖЕННЯ: Змінна TELEGRAM_CHAT_ID не встановлена.") } else {
		parsedChatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil { log.Printf("ПОМИЛКА: Не розпарсено TELEGRAM_CHAT_ID '%s': %v. ChatID=0.", chatIDStr, err) } else { chatID = parsedChatID }
	}

	cfg := Config{
		BotToken:         botToken,
		SpreadsheetID:    spreadsheetID,
		ChatID:           chatID,

		// Використовуємо getEnv для значень за замовчуванням
		SheetNameReport:         getEnv("SHEET_NAME_REPORT", "Звіт"),
		SheetRangeReport:        getEnv("SHEET_RANGE_REPORT", "A2:E2"), 
		SheetNameUserGoals:      getEnv("SHEET_NAME_USER_GOALS", "МоїЦілі"),
		SheetRangeUserGoals:     getEnv("SHEET_RANGE_USER_GOALS", "A:H"), 
		SheetNameWorkLog:        getEnv("SHEET_NAME_WORK_LOG", "РобочийГрафік"),
		SheetRangeWorkLogDates:  getEnv("SHEET_RANGE_WORK_LOG_DATES", "A:B"), 
		SheetRangeWorkLogFull:   getEnv("SHEET_RANGE_WORK_LOG_FULL", "A:E"),  
		SheetNameInvestments:    getEnv("SHEET_NAME_INVESTMENTS", "Інвестиції"), // <<< ЗАВАНТАЖЕННЯ НОВОГО ПОЛЯ

		WebhookBaseURL:    webhookBaseURL,
		WebhookPath:       webhookPath,
		WebhookListenAddr: getEnv("WEBHOOK_LISTEN_ADDR", ":443"), 
		WebhookCertPath:   os.Getenv("WEBHOOK_CERT_PATH"),       
		TLSCertPath:       tlsCertPath,
		TLSKeyPath:        tlsKeyPath,
	}

	// Додамо нове поле в лог
	log.Printf("Конфігурацію завантажено: SpreadsheetID=%s, ReportSheet='%s!%s', GoalsSheet='%s', WorkLogSheet='%s', InvestmentsSheet='%s', Webhook=%s%s, Listen=%s, AdminChatID=%d",
		cfg.SpreadsheetID, cfg.SheetNameReport, cfg.SheetRangeReport,
		cfg.SheetNameUserGoals, cfg.SheetNameWorkLog, cfg.SheetNameInvestments, // <<< Нове поле в лозі
		cfg.WebhookBaseURL, cfg.WebhookPath, cfg.WebhookListenAddr, cfg.ChatID)

	return cfg
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	log.Printf("ПОПЕРЕДЖЕННЯ: Змінна середовища %s не встановлена/порожня, використ. '%s'", key, fallback)
	return fallback
}
