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
	ChatID           int64 // Для адмінських повідомлень/звітів

	// Конфігурація Google Sheets
	SheetNameReport         string 
	SheetRangeReport        string 
	SheetNameUserGoals      string 
	SheetRangeUserGoals     string 
	SheetNameWorkLog        string 
	SheetRangeWorkLogDates  string 
	SheetRangeWorkLogFull   string 

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

	// Перевірка критично важливих змінних
	if botToken == "" { log.Fatal("Критична помилка: Змінна середовища TELEGRAM_TOKEN не встановлена.") }
	if spreadsheetID == "" { log.Fatal("Критична помилка: Змінна середовища SPREADSHEET_ID не встановлена.") }
	if webhookBaseURL == "" { log.Fatal("Критична помилка: Змінна середовища WEBHOOK_BASE_URL не встановлена.") }
	if webhookPath == "" { log.Fatal("Критична помилка: Змінна середовища WEBHOOK_PATH не встановлена.") }
	if tlsCertPath == "" { log.Fatal("Критична помилка: Змінна середовища TLS_CERT_PATH не встановлена.") }
	if tlsKeyPath == "" { log.Fatal("Критична помилка: Змінна середовища TLS_KEY_PATH не встановлена.") }

	// Завантаження та перевірка ChatID (з покращеним логуванням)
	var chatID int64 
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if chatIDStr == "" {
		// Якщо змінна не встановлена зовсім, це може бути нормально. Логуємо як попередження.
		log.Printf("ПОПЕРЕДЖЕННЯ: Змінна середовища TELEGRAM_CHAT_ID не встановлена. ChatID буде 0.")
		// chatID залишається 0 (нульове значення для int64)
	} else {
		// Якщо змінна встановлена, намагаємося її розпарсити.
		parsedChatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			// Якщо не вдалося розпарсити (напр., там текст замість числа) - це помилка конфігурації.
			log.Printf("ПОМИЛКА: Не вдалося розпарсити TELEGRAM_CHAT_ID '%s': %v. ChatID буде 0.", chatIDStr, err)
			// Вирішіть, чи є ця помилка критичною. Якщо ChatID обов'язковий, можна зробити log.Fatal тут.
			// Поки що залишаємо 0.
			chatID = 0 
		} else {
			chatID = parsedChatID // Присвоюємо розпарсений ID
		}
	}

	cfg := Config{
		BotToken:         botToken,
		SpreadsheetID:    spreadsheetID,
		ChatID:           chatID, // Використовуємо отриманий або нульовий chatID

		SheetNameReport:         getEnv("SHEET_NAME_REPORT", "Звіт"),
		SheetRangeReport:        getEnv("SHEET_RANGE_REPORT", "A2:E2"), 
		SheetNameUserGoals:      getEnv("SHEET_NAME_USER_GOALS", "МоїЦілі"),
		SheetRangeUserGoals:     getEnv("SHEET_RANGE_USER_GOALS", "A:H"), 
		SheetNameWorkLog:        getEnv("SHEET_NAME_WORK_LOG", "РобочийГрафік"),
		SheetRangeWorkLogDates:  getEnv("SHEET_RANGE_WORK_LOG_DATES", "A:B"), 
		SheetRangeWorkLogFull:   getEnv("SHEET_RANGE_WORK_LOG_FULL", "A:E"),  

		WebhookBaseURL:    webhookBaseURL,
		WebhookPath:       webhookPath,
		WebhookListenAddr: getEnv("WEBHOOK_LISTEN_ADDR", ":443"), 
		WebhookCertPath:   os.Getenv("WEBHOOK_CERT_PATH"),       
		TLSCertPath:       tlsCertPath,
		TLSKeyPath:        tlsKeyPath,
	}

	log.Printf("Конфігурацію завантажено: SpreadsheetID=%s, ReportSheet='%s!%s', GoalsSheet='%s', WorkLogSheet='%s', Webhook=%s%s, Listen=%s, AdminChatID=%d",
		cfg.SpreadsheetID, cfg.SheetNameReport, cfg.SheetRangeReport,
		cfg.SheetNameUserGoals, cfg.SheetNameWorkLog, cfg.WebhookBaseURL, cfg.WebhookPath, cfg.WebhookListenAddr, cfg.ChatID)

	return cfg
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	log.Printf("ПОПЕРЕДЖЕННЯ: Змінна середовища %s не встановлена або порожня, використовується '%s'", key, fallback)
	return fallback
}
