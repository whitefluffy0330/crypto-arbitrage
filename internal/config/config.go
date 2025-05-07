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
	ChatID           int64 // Можливо, для адмінських повідомлень або звітів за замовчуванням

	// Конфігурація Google Sheets (назви аркушів та діапазонів)
	SheetNameReport         string // Назва аркуша для зчитування звіту (наприклад, "Звіт")
	SheetRangeReport        string // Діапазон для зчитування звіту (наприклад, "A2:E2")
	SheetNameUserGoals      string // Назва аркуша для цілей користувачів (наприклад, "МоїЦілі")
	SheetRangeUserGoals     string // Діапазон для операцій з цілями (наприклад, "A:H")
	SheetNameWorkLog        string // Назва аркуша для робочого графіка (наприклад, "РобочийГрафік")
	SheetRangeWorkLogDates  string // Діапазон для читання дат/статусів робочого графіка (наприклад, "A:B")
	SheetRangeWorkLogFull   string // Діапазон для запису повного рядка робочого графіка (наприклад, "A:E")

	// Нові поля для Webhook та TLS
	WebhookBaseURL    string // Базовий URL для вебхука (https://your.domain.com) - ОБОВ'ЯЗКОВО
	WebhookPath       string // Секретний шлях для вебхука (напр., /hook/telegram_update_...) - ОБОВ'ЯЗКОВО
	WebhookListenAddr string // Адреса та порт для слухання вебхуків (напр., ":443" або ":8443")
	WebhookCertPath   string // Шлях до публічного сертифіката для SetWebhook (опціонально)
	TLSCertPath       string // Шлях до fullchain.pem для ListenAndServeTLS - ОБОВ'ЯЗКОВО для HTTPS
	TLSKeyPath        string // Шлях до privkey.pem для ListenAndServeTLS - ОБОВ'ЯЗКОВО для HTTPS
}

// LoadEnv завантажує конфігурацію зі змінних середовища
func LoadEnv() Config {
	// Завантаження існуючих змінних
	botToken := os.Getenv("TELEGRAM_TOKEN")
	spreadsheetID := os.Getenv("SPREADSHEET_ID")

	// Завантаження та перевірка ChatID
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil && chatIDStr != "" { // Якщо змінна встановлена, але не є числом
		log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося розпарсити TELEGRAM_CHAT_ID '%s': %v. Буде використано 0.", chatIDStr, err)
		chatID = 0 
	} else if chatIDStr == "" {
		log.Printf("ПОПЕРЕДЖЕННЯ: Змінна середовища TELEGRAM_CHAT_ID не встановлена.")
		chatID = 0 
	}

	// Нові обов'язкові змінні для Webhook/TLS
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

		WebhookBaseURL:    webhookBaseURL,
		WebhookPath:       webhookPath,
		WebhookListenAddr: getEnv("WEBHOOK_LISTEN_ADDR", ":443"), // Порт за замовчуванням 443
		WebhookCertPath:   os.Getenv("WEBHOOK_CERT_PATH"),        // Опціональний, може бути порожнім
		TLSCertPath:       tlsCertPath,
		TLSKeyPath:        tlsKeyPath,
	}

	log.Printf("Конфігурацію завантажено: SpreadsheetID=%s, ReportSheet='%s!%s', GoalsSheet='%s', WorkLogSheet='%s', Webhook=%s%s, Listen=%s",
		cfg.SpreadsheetID, cfg.SheetNameReport, cfg.SheetRangeReport,
		cfg.SheetNameUserGoals, cfg.SheetNameWorkLog, cfg.WebhookBaseURL, cfg.WebhookPath, cfg.WebhookListenAddr)

	return cfg
}

// getEnv допоміжна функція для отримання змінної середовища зі значенням за замовчуванням
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	log.Printf("ПОПЕРЕДЖЕННЯ: Змінна середовища %s не встановлена або порожня, використовується значення за замовчуванням: '%s'", key, fallback)
	return fallback
}
