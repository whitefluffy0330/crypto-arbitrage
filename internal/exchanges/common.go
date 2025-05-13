package exchanges

import "time"

// UnifiedFundingRateInfo містить уніфіковану інформацію про ставку фінансування з біржі
type UnifiedFundingRateInfo struct {
	Exchange        string    // Назва біржі (напр., "Binance", "Bybit")
	Symbol          string    // Торгова пара (напр., "BTCUSDT")
	MarkPrice       float64   // Ціна маркування
	LastFundingRate float64   // Остання ставка фінансування (у відсотках, напр., 0.01 для 0.01%)
	NextFundingTime time.Time // Час наступної виплати (UTC)
	// Можна додати інші поля, якщо вони універсальні, напр., IndexPrice, InterestRate
}