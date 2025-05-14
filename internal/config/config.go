// ... (існуючий код) ...

type Config struct {
	// ... існуючі поля ...
	SpreadCoinCount       int      `env:"SPREAD_COIN_COUNT,default=50"`
	SpreadMinPercentage   float64  `env:"SPREAD_MIN_PERCENTAGE,default=2.0"`
	SpreadUserExchanges   []string `env:"SPREAD_USER_EXCHANGES,default=binance,bybit"` // Розділені комою
	SpreadMinTrustScore   string   `env:"SPREAD_MIN_TRUST_SCORE,default=green"` // Може бути "green", "yellow", або числове значення як рядок
}

func LoadEnv() Config {
	// ... (існуючий код завантаження) ...
	
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

	spreadUserExchangesStr := getEnv("SPREAD_USER_EXCHANGES", "binance,bybit,okx,mexc,bitget") // Оновлено дефолт
	var spreadUserExchanges []string
	if spreadUserExchangesStr != "" {
		spreadUserExchanges = strings.Split(spreadUserExchangesStr, ",")
		for i, ex := range spreadUserExchanges { // Привести до нижнього регістру та прибрати пробіли
			spreadUserExchanges[i] = strings.ToLower(strings.TrimSpace(ex))
		}
	}
	
	spreadMinTrustScore := getEnv("SPREAD_MIN_TRUST_SCORE", "green") // green, yellow, red або порожній рядок для ігнорування


	cfg := Config{
		// ... існуючі поля ...
		SpreadCoinCount:       spreadCoinCount,
		SpreadMinPercentage:   spreadMinPercentage,
		SpreadUserExchanges:   spreadUserExchanges,
		SpreadMinTrustScore:   strings.ToLower(spreadMinTrustScore),
	}
	
	// ... (решта логування конфігурації) ...
	log.Printf("Конфігурацію завантажено: ... SpreadCoinCount=%d, SpreadMinPercentage=%.2f, SpreadUserExchanges=%v, SpreadMinTrustScore='%s'", 
		// ... існуючі поля ...
		cfg.SpreadCoinCount, cfg.SpreadMinPercentage, cfg.SpreadUserExchanges, cfg.SpreadMinTrustScore)

	return cfg
}
