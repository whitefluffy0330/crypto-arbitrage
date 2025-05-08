package telegram

import (
	"fmt"
	"log"
	"strings"
	"time" // Потрібен для форматування NextFundingTime

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	// Додаємо імпорт для binance
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/binance"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	gsheets "google.golang.org/api/sheets/v4"
)

const ( /* ... константи Callback... без змін ... */ 
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
	case "/goal", "🎯 Моя ціль": /* ... код без змін ... */
	case "/closegoal", "❌ Закрити ціль": /* ... код без змін ... */
	case "/add_investment": /* ... код без змін ... */
	
	// <<< НОВИЙ БЛОК ДЛЯ /funding >>>
	case "/funding", "💹 Funding Rates":
		log.Printf("Обробка команди /funding для ChatID: %d", chatID)
		
		// Повідомлення, що дані завантажуються
		loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Завантажую актуальні ставки фінансування з Binance...")
		sentMsg, _ := bot.Send(loadingMsg) // Не обробляємо помилку тут, щоб не переривати

		rates, err := binance.GetFundingRates()
		var fundingReportText string

		if err != nil {
			log.Printf("Помилка отримання funding rates: %v", err)
			fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати ставки фінансування: %v", err)
		} else {
			if len(rates) == 0 {
				fundingReportText = "Інформація про ставки фінансування наразі недоступна."
			} else {
				var sb strings.Builder
				sb.WriteString("📊 **Актуальні Ставки Фінансування (Binance Futures):**\n\n")
				
				// Виведемо декілька популярних пар для прикладу
				// У майбутньому можна додати фільтрацію або вибір символів
				symbolsToShow := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT"}
				shownCount := 0
				for _, symbol := range symbolsToShow {
					if info, ok := rates[symbol]; ok {
						// Переводимо час наступного фінансування у KyivLocation
						nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation)
						sb.WriteString(fmt.Sprintf("`%s`:\n", info.Symbol))
						sb.WriteString(fmt.Sprintf("  Mark Price: `%.4f`\n", info.MarkPrice))
						sb.WriteString(fmt.Sprintf("  Funding Rate: `%.4f%%`\n", info.LastFundingRate)) // Вже у відсотках з GetFundingRates
						sb.WriteString(fmt.Sprintf("  Наступна виплата: `%s` (за Києвом)\n\n", nextTimeKyiv.Format("15:04 02.01.2006")))
						shownCount++
					}
				}
				if shownCount == 0 {
					sb.WriteString("Не вдалося знайти дані для стандартних символів.")
				}
				fundingReportText = sb.String()
			}
		}
		// Оновлюємо повідомлення "Завантажую..." або надсилаємо нове
		if sentMsg.MessageID != 0 {
			editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, fundingReportText)
			editMsg.ParseMode = tgbotapi.ModeMarkdown
			bot.Send(editMsg)
		} else {
			finalMsg := tgbotapi.NewMessage(chatID, fundingReportText)
			finalMsg.ParseMode = tgbotapi.ModeMarkdown
			bot.Send(finalMsg)
		}
		keyboard.ShowMainKeyboard(bot, chatID) // Показуємо клавіатуру після дії

	case "/motivation": /* ... код без змін ... */
	case "/report", "📊 Прогрес": /* ... код без змін ... */
	default: /* ... код без змін ... */
	}
}
