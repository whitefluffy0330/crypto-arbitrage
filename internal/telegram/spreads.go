package telegram

import (
	"fmt"
	"log"
	"sort"
	"strconv" // Потрібен для strconv.Atoi у логіці отримання тікерів з CoinMarketCap
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coinmarketcap" // ДОДАНО ІМПОРТ
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
)

// SpreadOpportunity - існуюча структура для зберігання знайдених спредів
type SpreadOpportunity struct {
	CoinID          string
	BaseCurrency    string
	QuoteCurrency   string
	BuyExchange     string
	BuyPriceUSD     float64
	SellExchange    string
	SellPriceUSD    float64
	SpreadPercent   float64
	ProfitPer100USD float64
	TrustScoreBuy   string
	TrustScoreSell  string
	TradeURLBuy     string
	TradeURLSell    string
	Comment         string
	Category        int
}

// ---- Уніфіковані структури для обробки даних з різних джерел ----
type UnifiedCoin struct {
	ID     string // ID монети з джерела (CoinGecko ID або CMC ID)
	Symbol string // Символ монети (напр. BTC)
	Name   string // Назва монети (напр. Bitcoin)
	// DataSource string // Опціонально: "coingecko" або "coinmarketcap"
}

type UnifiedTicker struct {
	ExchangeName       string  // Повна назва біржі
	ExchangeIdentifier string  // Ідентифікатор біржі (coingecko identifier або cmc slug)
	BaseCurrency       string  // Базова валюта (напр. BTC)
	QuoteCurrency      string  // Валюта котирування (завжди USDT для нас)
	PriceUSD           float64 // Ціна в USD
	Volume24hUSD       float64 // Обсяг торгів за 24 години в USD (може бути 0, якщо немає даних)
	TrustScore         string  // Trust Score (напр. "green", "yellow", "red", "N/A")
	TradeURL           string  // Пряме посилання на торгову пару (може бути порожнім)
	// DataSource string // Опціонально
}
// ---- Кінець уніфікованих структур ----


// isUserExchange перевіряє, чи є біржа у списку користувацьких бірж.
// exchangeIdentifier має бути нормалізованим (lowercase, _ замість пробілів).
func isUserExchange(exchangeIdentifier string, userExchanges []string) bool {
	// userExchanges вже мають бути нормалізовані при завантаженні конфігурації.
	// Але для безпеки, можна нормалізувати й тут, якщо userExchanges не гарантовано нормалізовані.
	// normalizedIdentifier := strings.ToLower(strings.ReplaceAll(exchangeIdentifier, " ", "_"))
	for _, ue := range userExchanges {
		if exchangeIdentifier == ue { // Порівнюємо з вже нормалізованими значеннями з конфігу
			return true
		}
	}
	return false
}


// checkTrustScore перевіряє, чи відповідає TrustScore тікера мінімальним вимогам з конфігурації.
func checkTrustScore(tickerTrustScore string, minTrustScoreConfig string) bool {
	// Якщо фільтр не встановлено або "any", будь-який TrustScore підходить
	if minTrustScoreConfig == "" || minTrustScoreConfig == "any" {
		return true
	}

	// Якщо у тікера немає TrustScore, а фільтр встановлено (і він не "any"), то не підходить
	if tickerTrustScore == "" || tickerTrustScore == "N/A" { // Додано перевірку на "N/A"
		return false // Якщо є фільтр (не "any"), а score немає, то не проходить
	}

	// Нормалізуємо TrustScore тікера для порівняння
	normalizedTickerTrustScore := strings.ToLower(tickerTrustScore)

	switch strings.ToLower(minTrustScoreConfig) {
	case "green":
		return normalizedTickerTrustScore == "green"
	case "yellow":
		return normalizedTickerTrustScore == "green" || normalizedTickerTrustScore == "yellow"
	case "red": // "red" означає, що будь-який колір підходить (green, yellow, red)
		return normalizedTickerTrustScore == "green" || normalizedTickerTrustScore == "yellow" || normalizedTickerTrustScore == "red"
	default:
		log.Printf("Спреди: Невідомий або непідтримуваний формат SpreadMinTrustScore: '%s'. Фільтр TrustScore не застосовано для цього значення.", minTrustScoreConfig)
		return true // Якщо невідомий фільтр, вважаємо, що підходить
	}
}

// HandleSpreadsCommand шукає арбітражні спреди, використовуючи CoinGecko та CoinMarketCap як резервне джерело.
func HandleSpreadsCommand(bot *tgbotapi.BotAPI, chatID int64, cfg config.Config, originalMessageID int) {
	log.Printf("Спреди: Початок пошуку. Топ монет: %d, Мін. спред: %.2f%%. Джерела: CoinGecko, CoinMarketCap (fallback)", cfg.SpreadCoinCount, cfg.SpreadMinPercentage)

	var topCoins []UnifiedCoin // Використовуємо уніфіковану структуру
	var fetchError error
	var sourceUsedForCoins string = "Не визначено"

	// --- Стратегія: Спробувати CoinGecko, якщо невдача - CoinMarketCap ---
	log.Println("Спреди: Спроба отримати топ монет з CoinGecko...")
	cgCoins, cgErr := coingecko.GetTopMarketCapCoins(cfg.SpreadCoinCount, "usd")
	if cgErr != nil {
		log.Printf("Спреди: Помилка отримання топ-монет від CoinGecko: %v.", cgErr)
		fetchError = fmt.Errorf("CoinGecko: %v", cgErr) // Зберігаємо першу помилку

		if cfg.CoinMarketCapAPIKey != "" {
			log.Println("Спреди: Спроба отримати топ монет з CoinMarketCap...")
			sourceUsedForCoins = "CoinMarketCap"
			cmcCoins, cmcErr := coinmarketcap.GetTopMarketCapCoinsCMC(cfg.CoinMarketCapAPIKey, cfg.SpreadCoinCount)
			if cmcErr != nil {
				log.Printf("Спреди: Помилка отримання топ-монет від CoinMarketCap: %v", cmcErr)
				errorMsg := fmt.Sprintf("Помилка отримання списку топ-монет.\n%s\nCoinMarketCap: %v", fetchError, cmcErr)
				if strings.Contains(fetchError.Error(), "429") || (cmcErr != nil && (strings.Contains(cmcErr.Error(), "429") || strings.Contains(cmcErr.Error(), "rate limit"))) {
					errorMsg += "\n\n🚫 Схоже, ми досягли ліміту запитів до API. Спробуйте пізніше."
				}
				if originalMessageID != 0 {
					editMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, errorMsg)
					sendAndLog(bot, editMsg, "spreads_top_coins_error_edit_fallback", chatID)
				} else {
					sendAndLog(bot, tgbotapi.NewMessage(chatID, errorMsg), "spreads_top_coins_error_new_fallback", chatID)
				}
				keyboard.ShowMainKeyboard(bot, chatID)
				return
			}
			// Перетворення cmcCoins в []UnifiedCoin
			for _, coin := range cmcCoins {
				topCoins = append(topCoins, UnifiedCoin{ID: coin.ID, Symbol: coin.Symbol, Name: coin.Name})
			}
			log.Printf("Спреди: Отримано %d монет для аналізу з CoinMarketCap.", len(topCoins))
		} else {
			log.Println("Спреди: API ключ для CoinMarketCap не налаштовано. Неможливо використати як резервне джерело.")
			errorMsg := fmt.Sprintf("Помилка отримання списку топ-монет від CoinGecko: %v\n(CoinMarketCap недоступний: API ключ не надано)", cgErr) // Використовуємо cgErr тут
			if strings.Contains(cgErr.Error(), "429") {
				errorMsg += "\n\n🚫 Схоже, ми досягли ліміту запитів до CoinGecko API. Спробуйте пізніше."
			}
			if originalMessageID != 0 {
				editMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, errorMsg)
				sendAndLog(bot, editMsg, "spreads_top_coins_error_edit_cg_only", chatID)
			} else {
				sendAndLog(bot, tgbotapi.NewMessage(chatID, errorMsg), "spreads_top_coins_error_new_cg_only", chatID)
			}
			keyboard.ShowMainKeyboard(bot, chatID)
			return
		}
	} else {
		sourceUsedForCoins = "CoinGecko"
		// Успіх з CoinGecko, перетворюємо cgCoins в []UnifiedCoin
		for _, coin := range cgCoins {
			topCoins = append(topCoins, UnifiedCoin{ID: coin.ID, Symbol: coin.Symbol, Name: coin.Name})
		}
		log.Printf("Спреди: Отримано %d монет для аналізу з CoinGecko.", len(topCoins))
	}

	if len(topCoins) == 0 {
		log.Println("Спреди: Не отримано жодної монети для аналізу з доступних джерел.")
		noCoinsMsg := "Не вдалося отримати список монет для аналізу. Спробуйте пізніше."
		if originalMessageID != 0 {
			editMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, noCoinsMsg)
			sendAndLog(bot, editMsg, "spreads_no_coins_edit", chatID)
		} else {
			sendAndLog(bot, tgbotapi.NewMessage(chatID, noCoinsMsg), "spreads_no_coins_new", chatID)
		}
		keyboard.ShowMainKeyboard(bot, chatID)
		return
	}

	var allFoundSpreads []SpreadOpportunity
	userExchangesMap := make(map[string]bool)
	for _, ex := range cfg.SpreadUserExchanges { // cfg.SpreadUserExchanges вже мають бути нормалізовані
		userExchangesMap[ex] = true
	}

	processedCoins := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	delayBetweenCoinProcessing := 6 * time.Second // Затримка між обробкою різних МОНЕТ

	for _, coinLoopVar := range topCoins {
		wg.Add(1)
		go func(currentCoin UnifiedCoin, currentSourceForCoins string) {
			defer wg.Done()

			mu.Lock()
			processedCoins++
			currentProcessedLocal := processedCoins
			mu.Unlock()

			if originalMessageID != 0 && (currentProcessedLocal%3 == 0 || currentProcessedLocal == 1 || currentProcessedLocal == len(topCoins)) {
				progressText := fmt.Sprintf("⏳ Пошук спредів... Оброблено %d/%d: %s (%s) [Джерело монет: %s]", currentProcessedLocal, len(topCoins), currentCoin.Name, currentCoin.Symbol, currentSourceForCoins)
				editProgressMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, progressText)
				if _, errSend := bot.Send(editProgressMsg); errSend != nil {
					log.Printf("Спреди: Помилка оновлення прогресу: %v", errSend)
				}
			}

			var coinAllTickersForThisCoin []UnifiedTicker
			var tickerFetchErr error
			var sourceUsedForTickers string

			// Спроба отримати тікери з CoinGecko
			log.Printf("Спреди: Отримання тікерів для %s (ID: %s) з CoinGecko...", currentCoin.Name, currentCoin.ID)
			sourceUsedForTickers = "CoinGecko"
			cgTickersPageLimit := 1 // Обмежуємо однією сторінкою для CoinGecko через ліміти
			for page := 1; page <= cgTickersPageLimit; page++ {
				cgTickersResponse, cgTickerErr := coingecko.GetCoinTickers(currentCoin.ID, page)
				if cgTickerErr != nil {
					tickerFetchErr = fmt.Errorf("CoinGecko GetCoinTickers: %w", cgTickerErr)
					log.Printf("Спреди: Помилка CoinGecko GetCoinTickers для %s (ID: %s, стор. %d): %v", currentCoin.Name, currentCoin.ID, page, cgTickerErr)
					break
				}
				if len(cgTickersResponse.Tickers) == 0 {
					break
				}
				for _, ticker := range cgTickersResponse.Tickers {
					priceUSD, ok := ticker.ConvertedLast["usd"]
					if !ok { continue }
					volumeUSD, _ := ticker.ConvertedVolume["usd"] // Може бути 0, це нормально
					coinAllTickersForThisCoin = append(coinAllTickersForThisCoin, UnifiedTicker{
						ExchangeName:       ticker.Market.Name,
						ExchangeIdentifier: strings.ToLower(ticker.Market.Identifier), // Нормалізуємо ідентифікатор біржі
						BaseCurrency:       strings.ToUpper(ticker.Base),
						QuoteCurrency:      strings.ToUpper(ticker.Target),
						PriceUSD:           priceUSD,
						Volume24hUSD:       volumeUSD,
						TrustScore:         strings.ToLower(ticker.TrustScore), // Нормалізуємо TrustScore
						TradeURL:           ticker.TradeURL,
					})
				}
			}

			// Якщо з CoinGecko не вдалося або 0 тікерів, пробуємо CoinMarketCap
			if tickerFetchErr != nil || len(coinAllTickersForThisCoin) == 0 {
				log.Printf("Спреди: Невдача або 0 тікерів з CoinGecko для %s. Помилка: %v. Спроба CoinMarketCap...", currentCoin.Name, tickerFetchErr)
				if cfg.CoinMarketCapAPIKey != "" {
					sourceUsedForTickers = "CoinMarketCap"
					coinAllTickersForThisCoin = []UnifiedTicker{} // Очищаємо, якщо були часткові дані з CG
					
					// Для CMC використовуємо ID монети, якщо він числовий (тобто монета прийшла з CMC),
					// або символ монети, якщо ID нечисловий (монета прийшла з CG).
					cmcIdentifier := currentCoin.Symbol // За замовчуванням символ
					if _, errConv := strconv.Atoi(currentCoin.ID); errConv == nil && currentSourceForCoins == "CoinMarketCap" {
						cmcIdentifier = currentCoin.ID // Якщо монета з CMC, її ID підходить
					}
					
					log.Printf("Спреди: Отримання тікерів для %s (ідентифікатор для CMC: %s) з CoinMarketCap...", currentCoin.Name, cmcIdentifier)
					cmcTickers, cmcTickerErr := coinmarketcap.GetCoinTickersCMC(cfg.CoinMarketCapAPIKey, cmcIdentifier)
					if cmcTickerErr != nil {
						log.Printf("Спреди: Помилка CoinMarketCap GetCoinTickersCMC для %s (ідентифікатор: %s): %v", currentCoin.Name, cmcIdentifier, cmcTickerErr)
						return // Обидва джерела тікерів не вдалися
					}
					for _, ticker := range cmcTickers {
						coinAllTickersForThisCoin = append(coinAllTickersForThisCoin, UnifiedTicker{
							ExchangeName:       ticker.ExchangeName,
							ExchangeIdentifier: strings.ToLower(ticker.ExchangeIdentifier), // Нормалізуємо slug біржі
							BaseCurrency:       ticker.BaseCurrency, // Вже має бути ToUpper
							QuoteCurrency:      ticker.QuoteCurrency, // Вже має бути ToUpper
							PriceUSD:           ticker.PriceUSD,
							Volume24hUSD:       ticker.Volume24hUSD,
							TrustScore:         strings.ToLower(ticker.TrustScore), // "n/a" або інше, нормалізуємо
							TradeURL:           ticker.TradeURL,
						})
					}
					log.Printf("Спреди: Отримано %d тікерів для %s з CoinMarketCap.", len(coinAllTickersForThisCoin), currentCoin.Name)
				} else {
					log.Printf("Спреди: API ключ для CoinMarketCap не налаштовано. Неможливо отримати тікери для %s як резерв.", currentCoin.Name)
					// Якщо CG не вдався, і CMC недоступний, то для цієї монети тікерів не буде
					if len(coinAllTickersForThisCoin) == 0 { return }
				}
			}
			log.Printf("Спреди: Для монети %s (%s) знайдено %d тікерів з джерела: %s.", currentCoin.Name, currentCoin.Symbol, len(coinAllTickersForThisCoin), sourceUsedForTickers)

			var validTickers []UnifiedTicker
			for _, ticker := range coinAllTickersForThisCoin {
				if strings.ToUpper(ticker.QuoteCurrency) != "USDT" {
					continue
				}
				if !checkTrustScore(ticker.TrustScore, cfg.SpreadMinTrustScore) {
					continue
				}
				if ticker.PriceUSD <= 0 {
					continue
				}
				validTickers = append(validTickers, ticker)
			}
			
			if len(validTickers) < 2 {
				return
			}

			for i := 0; i < len(validTickers); i++ {
				for j := i + 1; j < len(validTickers); j++ {
					tickerA := validTickers[i]
					tickerB := validTickers[j]

					if tickerA.ExchangeIdentifier == tickerB.ExchangeIdentifier {
						continue
					}

					priceA_USD := tickerA.PriceUSD
					priceB_USD := tickerB.PriceUSD
					var buyTicker, sellTicker UnifiedTicker
					var spreadPercent, profitPer100 float64

					if priceA_USD < priceB_USD {
						buyTicker = tickerA
						sellTicker = tickerB
						if priceA_USD == 0 { continue }
						spreadPercent = (priceB_USD/priceA_USD - 1) * 100
						profitPer100 = 100 * (priceB_USD/priceA_USD - 1)
					} else if priceB_USD < priceA_USD {
						buyTicker = tickerB
						sellTicker = tickerA
						if priceB_USD == 0 { continue }
						spreadPercent = (priceA_USD/priceB_USD - 1) * 100
						profitPer100 = 100 * (priceA_USD/priceB_USD - 1)
					} else {
						continue
					}

					if spreadPercent >= cfg.SpreadMinPercentage {
						comment, category := classifySpreadSimplified(
							currentCoin.Symbol,
							buyTicker.ExchangeIdentifier,
							sellTicker.ExchangeIdentifier,
							userExchangesMap,
							validTickers,
							buyTicker.ExchangeName,
							sellTicker.ExchangeName,
						)
						
						if category == 4 && cfg.SpreadMinTrustScore != "" && cfg.SpreadMinTrustScore != "any" {
							// log.Printf("Спреди: Пропуск спреду категорії 4 для %s...", currentCoin.Symbol)
							continue
						}

						op := SpreadOpportunity{
							CoinID:          currentCoin.Symbol, // Використовуємо символ монети для відображення
							BaseCurrency:    strings.ToUpper(buyTicker.BaseCurrency),
							QuoteCurrency:   strings.ToUpper(buyTicker.QuoteCurrency),
							BuyExchange:     buyTicker.ExchangeName,
							BuyPriceUSD:     buyTicker.PriceUSD,
							SellExchange:    sellTicker.ExchangeName,
							SellPriceUSD:    sellTicker.PriceUSD,
							SpreadPercent:   spreadPercent,
							ProfitPer100USD: profitPer100,
							TrustScoreBuy:   buyTicker.TrustScore,
							TrustScoreSell:  sellTicker.TrustScore,
							TradeURLBuy:     buyTicker.TradeURL,
							TradeURLSell:    sellTicker.TradeURL,
							Comment:         comment,
							Category:        category,
						}
						mu.Lock()
						allFoundSpreads = append(allFoundSpreads, op)
						mu.Unlock()
					}
				}
			}
			// Затримка після обробки однієї монети (впливає на швидкість наступного запуску горутини для іншої монети)
			// Ця затримка тут потрібна, щоб не перевантажувати API запитами на тікери для РІЗНИХ монет.
			time.Sleep(delayBetweenCoinProcessing)

		}(coinLoopVar, sourceUsedForCoins) // Передаємо джерело, з якого була отримана монета
	}
	wg.Wait()

	sort.SliceStable(allFoundSpreads, func(i, j int) bool {
		if allFoundSpreads[i].Category != allFoundSpreads[j].Category {
			return allFoundSpreads[i].Category < allFoundSpreads[j].Category
		}
		return allFoundSpreads[i].SpreadPercent > allFoundSpreads[j].SpreadPercent
	})

	var reportText strings.Builder
	reportText.WriteString(fmt.Sprintf("📈 **Знайдені Спреди (мін. %.2f%%, Топ-%d монет):**\n", cfg.SpreadMinPercentage, cfg.SpreadCoinCount))
	reportText.WriteString("_Увага: Ціни з агрегаторів, можуть відрізнятися. Комісії не враховані._\n\n")

	if len(allFoundSpreads) == 0 {
		reportText.WriteString("Спредів, що відповідають вашим критеріям, не знайдено.")
	} else {
		limitSpreads := 7
		displayedCount := 0
		for _, s := range allFoundSpreads {
			if displayedCount >= limitSpreads {
				break
			}
			tsBuyDisplay := strings.ToUpper(s.TrustScoreBuy) // Відображаємо у верхньому регістрі для одноманітності
			if tsBuyDisplay == "" || tsBuyDisplay == "N/A" {
				tsBuyDisplay = "N/A"
			}
			tsSellDisplay := strings.ToUpper(s.TrustScoreSell)
			if tsSellDisplay == "" || tsSellDisplay == "N/A" {
				tsSellDisplay = "N/A"
			}

			reportText.WriteString(fmt.Sprintf(
				"**%s/%s (%.2f%%) | Прибуток на $100: `+$%.2f`**\n"+
					"  Купівля: *%s* (`$%.4f`, Trust: %s)\n"+
					"  Продаж: *%s* (`$%.4f`, Trust: %s)\n",
				s.BaseCurrency, s.QuoteCurrency, s.SpreadPercent, s.ProfitPer100USD,
				s.BuyExchange, s.BuyPriceUSD, tsBuyDisplay,
				s.SellExchange, s.SellPriceUSD, tsSellDisplay,
			))
			if s.Comment != "" {
				reportText.WriteString(fmt.Sprintf("  _%s_\n", s.Comment))
			}
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
			reportText.WriteString(fmt.Sprintf("... та ще %d спредів знайдено.", len(allFoundSpreads)-displayedCount))
		}
	}

	finalText := reportText.String()
	if len(finalText) > MaxTelegramMessageSize {
		log.Printf("Спреди: Повідомлення занадто довге (%d). Обрізаємо.", len(finalText))
		safeCutIndex := strings.LastIndex(finalText[:MaxTelegramMessageSize-30], "\n")
		if safeCutIndex == -1 || safeCutIndex < MaxTelegramMessageSize-100 {
			safeCutIndex = MaxTelegramMessageSize - 30
		}
		finalText = finalText[:safeCutIndex] + "\n... (повідомлення обрізано)"
	}

	if originalMessageID != 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, originalMessageID)
		if _, errDel := bot.Send(deleteMsg); errDel != nil {
			log.Printf("Спреди: Не вдалося видалити попереднє повідомлення (ID: %d): %v", originalMessageID, errDel)
		}
	}

	finalMsg := tgbotapi.NewMessage(chatID, finalText)
	finalMsg.ParseMode = tgbotapi.ModeMarkdown
	finalMsg.DisableWebPagePreview = true
	sendAndLog(bot, finalMsg, "spreads_report_final", chatID)

	keyboard.ShowMainKeyboard(bot, chatID)
}

// classifySpreadSimplified адаптована версія classifySpread.
func classifySpreadSimplified(
	coinSymbol string,
	exchangeBuyIdentifier string,
	exchangeSellIdentifier string,
	userExchangesMap map[string]bool,
	allTickersForCoin []UnifiedTicker,
	exchangeBuyName string,
	exchangeSellName string,
) (string, int) {
	buyIsUser := userExchangesMap[strings.ToLower(exchangeBuyIdentifier)] // Ідентифікатори вже мають бути lowercase
	sellIsUser := userExchangesMap[strings.ToLower(exchangeSellIdentifier)]

	tokenOnUserOtherExchange := false
	var userExchangeWithTokenName string

	for _, ticker := range allTickersForCoin {
		if strings.ToUpper(ticker.BaseCurrency) == strings.ToUpper(coinSymbol) &&
			strings.ToUpper(ticker.QuoteCurrency) == "USDT" &&
			userExchangesMap[strings.ToLower(ticker.ExchangeIdentifier)] && // ticker.ExchangeIdentifier вже lowercase
			strings.ToLower(ticker.ExchangeIdentifier) != strings.ToLower(exchangeBuyIdentifier) &&
			strings.ToLower(ticker.ExchangeIdentifier) != strings.ToLower(exchangeSellIdentifier) {
			tokenOnUserOtherExchange = true
			userExchangeWithTokenName = ticker.ExchangeName
			break
		}
	}

	if buyIsUser && sellIsUser {
		return "✅ Обидві біржі у вашому списку!", 1
	}
	if buyIsUser {
		if tokenOnUserOtherExchange {
			return fmt.Sprintf("ℹ️ Купівля на вашій біржі (%s). Продаж на '%s' (не ваша). Токен також є на вашій біржі '%s'.", exchangeBuyName, exchangeSellName, userExchangeWithTokenName), 2
		}
		return fmt.Sprintf("⚠️ Купівля на вашій біржі (%s). Продаж на '%s' (не ваша). Цього токена немає на інших ваших біржах.", exchangeBuyName, exchangeSellName), 2
	}
	if sellIsUser {
		if tokenOnUserOtherExchange {
			return fmt.Sprintf("ℹ️ Продаж на вашій біржі (%s). Купівля на '%s' (не ваша). Токен також є на вашій біржі '%s'.", exchangeSellName, exchangeBuyName, userExchangeWithTokenName), 2
		}
		return fmt.Sprintf("⚠️ Продаж на вашій біржі (%s). Купівля на '%s' (не ваша). Цього токена немає на інших ваших біржах.", exchangeSellName, exchangeBuyName), 2
	}
	if tokenOnUserOtherExchange {
		return fmt.Sprintf("🔍 Спред між '%s' та '%s' (не ваші). Токен є на вашій біржі '%s'.", exchangeBuyName, exchangeSellName, userExchangeWithTokenName), 3
	}
	return fmt.Sprintf("🚫 Спред між '%s' та '%s' (не ваші). Токена немає на ваших біржах.", exchangeBuyName, exchangeSellName), 4
}


// min використовується для обмеження довжини повідомлення в логах
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
