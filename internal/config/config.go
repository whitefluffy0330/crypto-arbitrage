package config

import (
	"log" // Додано для логування, якщо змінні не встановлено
	"os"
	"strconv"
)

// Config зберігає всі конфігураційні параметри застосунку
type Config struct {
	BotToken         string
	SpreadsheetID    string
	ChatID           int64 // Можливо, для адмінських повідомлень або звітів за замовчуванням

	// Нові поля для назв аркушів та діапазонів
	SheetNameReport         string // Назва аркуша для зчитування звіту (наприклад, "Звіт")
	SheetRangeReport        string // Діапазон для зчитування звіту (наприклад, "A2:E2")
	SheetNameUserGoals      string // Назва аркуша для цілей користувачів (наприклад, "МоїЦілі")
	SheetRangeUserGoals     string // Діапазон для операцій з цілями (наприклад, "A:H")
	SheetNameWorkLog        string // Назва аркуша для робочого графіка (наприклад, "РобочийГрафік")
	SheetRangeWorkLogDates  string // Діапазон для читання дат/статусів робочого графіка (наприклад, "A:B")
	SheetRangeWorkLogFull   string // Діапазон для запису повного рядка робочого графіка (наприклад, "A:E")
}

// LoadEnv завантажує конфігурацію зі змінних середовища
func LoadEnv() Config {
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil && chatIDStr != "" { // Якщо змінна встановлена, але не є числом
		log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося розпарсити TELEGRAM_CHAT_ID '%s': %v. Буде використано 0.", chatIDStr, err)
		chatID = 0 // Встановлюємо значення за замовчуванням або обробляємо як критичну помилку, якщо потрібно
	} else if chatIDStr == "" {
		// Якщо TELEGRAM_CHAT_ID не встановлено зовсім, це може бути не критично для всіх функцій
		log.Printf("ПОПЕРЕДЖЕННЯ: Змінна середовища TELEGRAM_CHAT_ID не встановлена. Деякі функції можуть працювати некоректно.")
		chatID = 0 
	}


	cfg := Config{
		BotToken:      os.Getenv("TELEGRAM_TOKEN"),
		SpreadsheetID: os.Getenv("SPREADSHEET_ID"),
		ChatID:        chatID,

		// Завантажуємо нові налаштування з можливістю значень за замовчуванням
		SheetNameReport:         getEnv("SHEET_NAME_REPORT", "Звіт"),
		SheetRangeReport:        getEnv("SHEET_RANGE_REPORT", "A2:E2"), // Стандартний діапазон
		SheetNameUserGoals:      getEnv("SHEET_NAME_USER_GOALS", "МоїЦілі"),
		SheetRangeUserGoals:     getEnv("SHEET_RANGE_USER_GOALS", "A:H"), // Покриває всі колонки цілей
		SheetNameWorkLog:        getEnv("SHEET_NAME_WORK_LOG", "РобочийГрафік"),
		SheetRangeWorkLogDates:  getEnv("SHEET_RANGE_WORK_LOG_DATES", "A:B"), // Для читання дат і статусів
		SheetRangeWorkLogFull:   getEnv("SHEET_RANGE_WORK_LOG_FULL", "A:E"),  // Для запису повного рядка
	}

	// Логування завантаженої конфігурації (без токена)
	log.Printf("Конфігурацію завантажено: SpreadsheetID=%s, ReportSheet='%s!%s', GoalsSheet='%s!%s', WorkLogSheet='%s'",
		cfg.SpreadsheetID, cfg.SheetNameReport, cfg.SheetRangeReport,
		cfg.SheetNameUserGoals, cfg.SheetRangeUserGoals, cfg.SheetNameWorkLog)

	return cfg
}

// getEnv допоміжна функція для отримання змінної середовища зі значенням за замовчуванням
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	log.Printf("ПОПЕРЕДЖЕННЯ: Змінна середовища %s не встановлена, використовується значення за замовчуванням: '%s'", key, fallback)
	return fallback
}
