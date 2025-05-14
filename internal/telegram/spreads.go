package telegram

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
)

// SpreadOpportunity зберігає інформацію про знайдений спред
type SpreadOpportunity struct {
	CoinID         string  // ID монети на CoinGecko (наприклад, "bitcoin")
	BaseCurrency   string  // Символ базової валюти (наприклад, "BTC")
	QuoteCurrency  string  // Символ котирувальної валюти (наприклад, "USDT")
	BuyExchange    string  // Назва біржі для купівлі
	BuyPriceUSD    float64 // Ціна купівлі в USD
	SellExchange   string  // Назва біржі для продажу
	SellPriceUSD   float64 // Ціна продажу в USD
	SpreadPercent  float64 // Розрахований відсоток спреду
	TrustScoreBuy  string  // Рейтинг надійності біржі купівлі ("green", "yellow", "red", "" якщо немає)
	TrustScoreSell string  // Рейтинг надійності біржі продажу
	TradeURLBuy    string  // Пряме посилання на торгову пару для купівлі
	TradeURLSell   string  // Пряме посилання на торгову пару для продажу
	Comment        string  // Коментар щодо "ваших бірж" та наявності токена
	Category       int     // Категорія пріоритету (1-найвищий, 2, 3)
}

// isUserExchange перевіряє, чи є біржа у списку користувацьких
// Ідентифікатори бірж з CoinGecko зазвичай в нижньому регістрі та використовують "_" замість пробілів
func isUserExchange(exchangeIdentifier string, userExchanges []string) bool {
	normalizedIdentifier := strings.ToLower(strings.ReplaceAll(exchangeIdentifier, " ", "_"))
	for _, ue := range userExchanges { // userExchanges вже мають бути в нижньому регістрі з config
		if normalizedIdentifier == ue {
			return true
		}
	}
	return false
}

// checkTrustScore перевіряє, чи проходить біржа фільтр за Trust Score
func checkTrustScore(tickerTrustScore string, minTrustScoreConfig string) bool {
	if minTrustScoreConfig == "" || minTrustScoreConfig == "any" {
		return true
	}
	// CoinGecko повертає "green", "yellow", "red" або nil (якщо немає даних)
	// Важливо: ticker.TrustScore може бути порожнім рядком, якщо CoinGecko не надав його
	if tickerTrustScore == "" && minTrustScoreConfig != "" && minTrustScoreConfig != "any" { // Якщо потрібен певний скор, а його немає - не проходить
		return false
	}

	switch strings.ToLower(minTrustScoreConfig) {
	case "green":
		return strings.ToLower(tickerTrustScore) == "green"
	case "yellow":
		return strings.ToLower(tickerTrustScore) == "green" || strings.ToLower(tickerTrustScore) == "yellow"
	case "red": // Включає всі, включно з червоним
		return true
	default:
		// Спроба парсити числове значення (якщо CoinGecko колись змінить на числа 1-10)
		// Але поточна документація вказує на рядкові значення.
		// Поки що, якщо не green/yellow/red/any/"" - вважаємо, що фільтр не застосовується.
		return true
	}
}

// classifySpread визначає категорію спреду та формує коментар
// exchangeBuyID та exchangeSellID - це market.identifier з CoinGecko
func classifySpread(coinSymbol string, exchangeBuyID, exchangeSellID string, userExchangesMap map[string]bool, allCoinGeckoTickers []coingecko.CoinGeckoTickerDetail) (string, int) {
	buyIsUser := userExchangesMap[strings.ToLower(exchangeBuyID)]
	sellIsUser := userExchangesMap[strings.ToLower(exchangeSellID)]

	// Перевірка, чи торгується монета на "моїх" біржах (окрім тих, що вже в спреді)
	tokenOnUserOtherExchange := false
	var userExchangeWithToken string
	for _, ticker := range allCoinGeckoTickers {
		if strings.ToUpper(ticker.Base) == strings.ToUpper(coinSymbol) && // Перевіряємо базовий символ
			strings.ToUpper(ticker.Target) == "USDT" && // І що це пара до USDT
			userExchangesMap[strings.ToLower(ticker.Market.Identifier)] && // І це "моя" біржа
			strings.ToLower(ticker.Market.Identifier) != strings.ToLower(exchangeBuyID) && // І це не та біржа, де купуємо
			strings.ToLower(ticker.Market.Identifier) != strings.ToLower(exchangeSellID) { // І це не та біржа, де продаємо
			tokenOnUserOtherExchange = true
			userExchangeWithToken = ticker.Market.Name
			break
		}
	}

	if buyIsUser && sellIsUser {
		return "✅ Обидві біржі у вашому списку!", 1
	}
	if buyIsUser { // Купівля на "моїй", продаж на "чужій"
		if tokenOnUserOtherExchange {
			return fmt.Sprintf("ℹ️ Купівля на вашій біржі. Продаж на '%s' (не ваша). Токен також є на вашій біржі '%s'.", exchangeSellID, userExchangeWithToken), 2
		}
		return fmt.Sprintf("⚠️ Купівля на вашій біржі. Продаж на '%s' (не ваша). Токена немає на інших ваших біржах для можливого повернення.", exchangeSellID), 2
	}
	if sellIsUser { // Продаж на "моїй", купівля на "чужій"
		if tokenOnUserOtherExchange {
			return fmt.Sprintf("ℹ️ Продаж на вашій біржі. Купівля на '%s' (не ваша). Токен також є на вашій біржі '%s'.", exchangeBuyID, userExchangeWithToken), 2
		}
		return fmt.Sprintf("⚠️ Продаж на вашій біржі. Купівля на '%s' (не ваша). Токена немає на інших ваших біржах для можливого арбітражу.", exchangeBuyID), 2
	}
	// Жодна з бірж у спреді не "моя"
	if tokenOnUserOtherExchange {
		return fmt.Sprintf("🔍 Спред між '%s' та '%s' (не ваші). Токен є на вашій біржі '%s'.", exchangeBuyID, exchangeSellID, userExchangeWithToken), 3
	}
	return fmt.Sprintf("🚫 Спред між '%s' та '%s' (не ваші). Токена немає на ваших біржах.", exchangeBuyID, exchangeSellID), 4
}

// HandleSpreadsCommand обробляє запит на пошук спредів
func HandleSpreadsCommand(bot *tgbotapi.BotAPI, chatID int64, cfg config.Config) {
	loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Пошук спредів... Це може зайняти деякий час (до кількох хвилин), будь ласка, зачекайте.")
	sentMsg, errSendLoad := bot.Send(loadingMsg)
	var originalMessageID int
	if errSendLoad == nil && sentMsg.MessageID != 0 {
		originalMessageID = sentMsg.MessageID
	} else {
		log.Printf("Спреди: Помилка надсилання повідомлення 'Пошук спредів': %v", errSendLoad)
	}

	log.Printf("Спреди: Початок пошуку. Топ монет: %d, Мін. спред: %.2f%%, Біржі користувача: %v, Мін. Trust Score: '%s'",
		cfg.SpreadCoinCount, cfg.SpreadMinPercentage, cfg.SpreadUserExchanges, cfg.SpreadMinTrustScore)

	topCoins, err := coingecko.GetTopMarketCapCoins(cfg.SpreadCoinCount, "usd")
	if err != nil {
		log.Printf("Спреди: Помилка отримання топ монет: %v", err)
		errorText := fmt.Sprintf("Помилка отримання списку топ-монет: %v", err)
		if originalMessageID != 0 {
			editMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, errorText)
			sendAndLog(bot, editMsg, "spreads_top_coins_error_edit", chatID)
		} else {
			sendAndLog(bot, tgbotapi.NewMessage(chatID, errorText), "spreads_top_coins_error_new", chatID)
		}
		keyboard.ShowMainKeyboard(bot, chatID) // Завжди показуємо клавіатуру після завершення
		return
	}
	log.Printf("Спреди: Отримано %d монет для аналізу.", len(topCoins))

	var allFoundSpreads []SpreadOpportunity
	userExchangesMap := make(map[string]bool)
	for _, ex := range cfg.SpreadUserExchanges { // cfg.SpreadUserExchanges вже мають бути в нижньому регістрі
		userExchangesMap[ex] = true
	}

	processedCoins := 0
	var mu sync.Mutex // Для безпечного доступу до allFoundSpreads з горутин
	var wg sync.WaitGroup

	// Обмеження кількості одночасних горутин для CoinGecko API
	// CoinGecko має ліміт близько 10-30 запитів на хвилину для безкоштовного API
	// Ми будемо робити 1 запит GetTopMarketCapCoins + N_coins * (в середньому 1-2 сторінки GetCoinTickers)
	// Це може бути багато запитів, тому обережно.
	// Для початку, спробуємо без обмеження горутин, але з таймаутами в GetCoinTickers.
	// Якщо будуть проблеми з лімітами, доведеться додавати семафор.

	for _, coin := range topCoins {
		wg.Add(1)
		go func(c coingecko.CoinMarketData) {
			defer wg.Done()

			mu.Lock() // Блокуємо для безпечного оновлення processedCoins та надсилання повідомлення
			processedCoins++
			currentProcessed := processedCoins
			mu.Unlock()

			if originalMessageID != 0 && currentProcessed%5 == 0 {
				progressText := fmt.Sprintf("⏳ Пошук спредів... Оброблено %d з %d монет (%s)...", currentProcessed, len(topCoins), c.Name)
				editProgressMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, progressText)
				_, _ = bot.Send(editProgressMsg)
			}

			// log.Printf("Спреди: Обробка монети %s (%s)", c.Name, c.ID)

			var coinAllTickers []coingecko.CoinGeckoTickerDetail // Збираємо всі тікери для монети тут
			for page := 1; page <= 3; page++ {
				tickersResponse, errTicker := coingecko.GetCoinTickers(c.ID, page)
				if errTicker != nil {
					// log.Printf("Спреди: Помилка отримання тікерів для %s (сторінка %d): %v", c.ID, page, errTicker)
					break
				}
				if len(tickersResponse.Tickers) == 0 {
					break
				}
				coinAllTickers = append(coinAllTickers, tickersResponse.Tickers...)
				if len(tickersResponse.Tickers) < 100 {
					break
				}
				time.Sleep(1200 * time.Millisecond) // Пауза між запитами сторінок однієї монети
			}
			// log.Printf("Спреди: Отримано %d тікерів для %s.", len(coinAllTickers), c.Name)

			var validTickers []coingecko.CoinGeckoTickerDetail
			for _, ticker := range coinAllTickers {
				if strings.ToUpper(ticker.Target) != "USDT" {
					continue
				}
				if !checkTrustScore(ticker.TrustScore, cfg.SpreadMinTrustScore) {
					continue
				}
				if priceUSD, ok := ticker.ConvertedLast["usd"]; ok && priceUSD > 0 {
					validTickers = append(validTickers, ticker)
				}
			}

			if len(validTickers) < 2 { // Потрібно хоча б два тікери для порівняння
				return
			}

			for i := 0; i < len(validTickers); i++ {
				for j := i + 1; j < len(validTickers); j++ {
					tickerA := validTickers[i]
					tickerB := validTickers[j]

					if tickerA.Market.Identifier == tickerB.Market.Identifier {
						continue
					}

					priceA_USD := tickerA.ConvertedLast["usd"]
					priceB_USD := tickerB.ConvertedLast["usd"]

					// Спред: купуємо на дешевшій, продаємо на дорожчій
					var buyTicker, sellTicker coingecko.CoinGeckoTickerDetail
					var spreadPercent float64

					if priceA_USD < priceB_USD { // Купуємо на A, продаємо на B
						buyTicker = tickerA
						sellTicker = tickerB
						spreadPercent = (priceB_USD/priceA_USD - 1) * 100
					} else if priceB_USD < priceA_USD { // Купуємо на B, продаємо на A
						buyTicker = tickerB
						sellTicker = tickerA
						spreadPercent = (priceA_USD/priceB_USD - 1) * 100
					} else {
						continue // Ціни однакові
					}

					if spreadPercent >= cfg.SpreadMinPercentage {
						comment, category := classifySpread(c.Symbol, buyTicker.Market.Identifier, sellTicker.Market.Identifier, userExchangesMap, coinAllTickers)

						// Категорія 4 (найнижчий пріоритет) - не додаємо, якщо не хочемо їх бачити
						// if category == 4 { continue }

						op := SpreadOpportunity{
							CoinID: c.ID, BaseCurrency: strings.ToUpper(buyTicker.Base), QuoteCurrency: strings.ToUpper(buyTicker.Target),
							BuyExchange: buyTicker.Market.Name, BuyPriceUSD: buyTicker.ConvertedLast["usd"],
							SellExchange: sellTicker.Market.Name, SellPriceUSD: sellTicker.ConvertedLast["usd"],
							SpreadPercent: spreadPercent,
							TrustScoreBuy: buyTicker.TrustScore, TrustScoreSell: sellTicker.TrustScore,
							TradeURLBuy: buyTicker.TradeURL, TradeURLSell: sellTicker.TradeURL,
							Comment: comment, Category: category,
						}
						mu.Lock()
						allFoundSpreads = append(allFoundSpreads, op)
						mu.Unlock()
					}
				}
			}
			// Невелика пауза між обробкою різних монет, щоб не перевантажувати CoinGecko
			time.Sleep(2 * time.Second)
		}(coin)
	}
	wg.Wait() // Чекаємо завершення всіх горутин

	sort.SliceStable(allFoundSpreads, func(i, j int) bool {
		if allFoundSpreads[i].Category != allFoundSpreads[j].Category {
			return allFoundSpreads[i].Category < allFoundSpreads[j].Category
		}
		return allFoundSpreads[i].SpreadPercent > allFoundSpreads[j].SpreadPercent
	})

	var reportText strings.Builder
	reportText.WriteString(fmt.Sprintf("📈 **Знайдені Спреди (мін. %.2f%%, Топ-%d монет):**\n\n", cfg.SpreadMinPercentage, cfg.SpreadCoinCount))

	if len(allFoundSpreads) == 0 {
		reportText.WriteString("Спредів, що відповідають вашим критеріям, не знайдено.")
	} else {
		limitSpreads := 7 // Обмеження на кількість спредів у повідомленні (можна зробити конфігурованим)
		displayedCount := 0
		for _, s := range allFoundSpreads {
			if displayedCount >= limitSpreads {
				break
			}
			// Формуємо рядок для TrustScore
			tsBuyDisplay := s.TrustScoreBuy
			if tsBuyDisplay == "" {
				tsBuyDisplay = "N/A"
			}
			tsSellDisplay := s.TrustScoreSell
			if tsSellDisplay == "" {
				tsSellDisplay = "N/A"
			}

			reportText.WriteString(fmt.Sprintf(
				"**%s/%s (%.2f%%)**\n"+
					"  Купівля: *%s* (`$%.4f`, Trust: %s)\n"+
					"  Продаж: *%s* (`$%.4f`, Trust: %s)\n",
				s.BaseCurrency, s.QuoteCurrency, s.SpreadPercent,
				s.BuyExchange, s.BuyPriceUSD, tsBuyDisplay,
				s.SellExchange, s.SellPriceUSD, tsSellDisplay,
			))
			if s.Comment != "" {
				reportText.WriteString(fmt.Sprintf("  _%s_\n", s.Comment))
			}
			// Додаємо посилання тільки якщо вони є
			buyLink := ""
			if s.TradeURLBuy != "" {
				buyLink = fmt.Sprintf("[Купити](%s)", s.TradeURLBuy)
			}
			sellLink := ""
			if s.TradeURLSell != "" {
				sellLink = fmt.Sprintf("[Продати](%s)", s.TradeURLSell)
			}

			if buyLink != "" && sellLink != "" {
				reportText.WriteString(fmt.Sprintf("  %s | %s\n", buyLink, sellLink))
			} else if buyLink != "" {
				reportText.WriteString(fmt.Sprintf("  %s\n", buyLink))
			} else if sellLink != "" {
				reportText.WriteString(fmt.Sprintf("  %s\n", sellLink))
			}
			reportText.WriteString("\n")
			displayedCount++
		}
		if len(allFoundSpreads) > displayedCount {
			reportText.WriteString(fmt.Sprintf("... та ще %d спредів знайдено (перевірте логи для повного списку).", len(allFoundSpreads)-displayedCount))
		}
	}

	finalText := reportText.String()
	if len(finalText) > MaxTelegramMessageSize {
		log.Printf("Спреди: Повідомлення занадто довге (%d). Обрізаємо.", len(finalText))
		finalText = finalText[:MaxTelegramMessageSize-30] + "\n... (повідомлення обрізано)"
	}

	// Видаляємо повідомлення "Завантажую..." та надсилаємо нове зі звітом
	if originalMessageID != 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, originalMessageID)
		_, _ = bot.Send(deleteMsg) // Помилку не обробляємо критично
	}

	finalMsg := tgbotapi.NewMessage(chatID, finalText)
	finalMsg.ParseMode = tgbotapi.ModeMarkdown
	finalMsg.DisableWebPagePreview = true // Вимикаємо попередній перегляд посилань, щоб повідомлення було компактнішим
	sendAndLog(bot, finalMsg, "spreads_report_final", chatID)

	keyboard.ShowMainKeyboard(bot, chatID)
}

// min - допоміжна функція
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
