package telegram

import (
	"fmt"
	"log"
	"sort" // Додано для сортування
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/binance"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

const (
	CallbackConfirmCloseGoal = "confirm_close_goal"
	CallbackCancelCloseGoal  = "cancel_close_goal"
)

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	if update.CallbackQuery != nil { /* ... код обробки CallbackQuery без змін ... */ return }
	if update.Message == nil { return }
	chatID := update.Message.Chat.ID; msgText := update.Message.Text; userName := update.Message.From.UserName
	log.Printf("[%s] (%d): %s", userName, chatID, msgText)
	currentState := GetUserState(chatID)

	if currentState == StateAwaitingGoalInput { /* ... код керування станом для цілі без змін ... */ } else if currentState == StateAwaitingInvestmentInput { /* ... код керування станом для інвестиції без змін ... */ }
	
	switch msgText {
	case "/start", "🔁 Старт": commands.StartWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/stop", "⛔️ Стоп": commands.StopWork(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/dayoff", "🏖 Вихідний": commands.DayOff(bot, update.Message, srv, cfg); keyboard.ShowMainKeyboard(bot, chatID) 
	case "/goal", "🎯 Моя ціль": /* ... код без змін з #199 ... */
	case "/closegoal", "❌ Закрити ціль": /* ... код без змін з #199 ... */
	case "/add_investment": /* ... код без змін з #199 ... */
	
	case "/funding", "💹 Funding Rates":
		log.Printf("Обробка команди /funding для ChatID: %d", chatID)
		loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Завантажую та сортую ставки фінансування з Binance...")
		sentMsg, _ := bot.Send(loadingMsg)

		allRatesMap, err := binance.GetFundingRates()
		var fundingReportText string

		if err != nil {
			log.Printf("Помилка отримання funding rates: %v", err)
			fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати ставки: %v", err)
		} else {
			if len(allRatesMap) == 0 {
				fundingReportText = "Інформація про ставки фінансування наразі недоступна."
			} else {
				// Конвертуємо мапу в зріз для сортування
				var allRatesSlice []binance.FundingInfo
				for _, rateInfo := range allRatesMap {
					allRatesSlice = append(allRatesSlice, rateInfo)
				}

				// Сортуємо: спочатку позитивні (від більшої до меншої), потім негативні (від найменшої до найбільшої по модулю)
				sort.SliceStable(allRatesSlice, func(i, j int) bool {
					// Спочатку ті, що з вищим позитивним фінансуванням
					if allRatesSlice[i].LastFundingRate > 0 && allRatesSlice[j].LastFundingRate <= 0 {
						return true
					}
					if allRatesSlice[i].LastFundingRate <= 0 && allRatesSlice[j].LastFundingRate > 0 {
						return false
					}
					// Якщо обидві позитивні, то більша перша
					if allRatesSlice[i].LastFundingRate > 0 && allRatesSlice[j].LastFundingRate > 0 {
						return allRatesSlice[i].LastFundingRate > allRatesSlice[j].LastFundingRate
					}
					// Якщо обидві негативні (або 0), то та, що менша (більш негативна), перша
					return allRatesSlice[i].LastFundingRate < allRatesSlice[j].LastFundingRate
				})
				
				var sb strings.Builder
				sb.WriteString("📊 **Топ Ставки Фінансування (Binance Futures):**\n_(оцінка для позиції $100 за 8 год.)_\n\n")
				
				limit := 7 // Скільки пар показувати з кожного краю (позитивні/негативні)
				count := 0

				// Позитивні ставки (вигідно для Short)
				sb.WriteString("📈 **Найвищі Позитивні Ставки (вигідно Short):**\n")
				foundPositive := false
				for _, info := range allRatesSlice {
					if info.LastFundingRate > 0.001 { // Показуємо лише ті, що трохи значущі
						profitPer100 := info.LastFundingRate // Ставка вже у %, тому це і є $ на $100
						nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation)
						sb.WriteString(fmt.Sprintf("`%s`: `%.4f%%` (+$%.2f) (Next: %s)\n", info.Symbol, info.LastFundingRate, profitPer100, nextTimeKyiv.Format("15:04")))
						count++
						foundPositive = true
						if count >= limit { break }
					}
				}
				if !foundPositive { sb.WriteString("_Не знайдено значних позитивних ставок._\n") }
				sb.WriteString("\n")

				// Негативні ставки (вигідно для Long)
				sb.WriteString("📉 **Найбільш Негативні Ставки (вигідно Long):**\n")
				count = 0 // Скидаємо лічильник
				foundNegative := false
				// Шукаємо з кінця відсортованого масиву для найбільш негативних
				for i := len(allRatesSlice) - 1; i >= 0; i-- {
					info := allRatesSlice[i]
					if info.LastFundingRate < -0.001 { // Показуємо лише ті, що трохи значущі
						// Для негативних ставок, виплата (позитивна для лонга) буде |ставка| * $100
						payoutPer100 := -info.LastFundingRate // Беремо абсолютне значення для "вигоди"
						nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation)
						sb.WriteString(fmt.Sprintf("`%s`: `%.4f%%` (+$%.2f) (Next: %s)\n", info.Symbol, info.LastFundingRate, payoutPer100, nextTimeKyiv.Format("15:04")))
						count++
						foundNegative = true
						if count >= limit { break }
					}
				}
				if !foundNegative { sb.WriteString("_Не знайдено значних негативних ставок._\n") }
				fundingReportText = sb.String()
			}
		}
		// Оновлюємо повідомлення
		if sentMsg.MessageID != 0 { editText := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, fundingReportText); editText.ParseMode = tgbotapi.ModeMarkdown; bot.Send(editText) } else { finalMsg := tgbotapi.NewMessage(chatID, fundingReportText); finalMsg.ParseMode = tgbotapi.ModeMarkdown; bot.Send(finalMsg) }
		keyboard.ShowMainKeyboard(bot, chatID)

	case "/motivation": motivationText := motivation.GetRandomMotivation(); msg := tgbotapi.NewMessage(chatID, motivationText); if _, err := bot.Send(msg); err != nil { log.Printf("Помилка мотивації: %v", err) }; keyboard.ShowMainKeyboard(bot, chatID) 
	case "/report", "📊 Прогрес": ReportProgress(bot, update.Message, srv, cfg) 
	default: log.Printf("Не розпізнана команда: [%s]: %s.", userName, msgText); keyboard.ShowMainKeyboard(bot, chatID)
	}
}
