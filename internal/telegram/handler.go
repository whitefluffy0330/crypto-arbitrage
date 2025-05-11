package telegram

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time" // Потрібен для time.Until та форматування часу

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/binance"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/coingecko"
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

// Допоміжні функції для логування надсилання повідомлень
func sendAndLog(bot *tgbotapi.BotAPI, c tgbotapi.Chattable, commandName string, chatID int64) {
	if _, err := bot.Send(c); err != nil {
		log.Printf("ПОМИЛКА надсилання (%s) для %d: %v", commandName, chatID, err)
	} else {
		log.Printf("Надіслано відповідь (%s) для %d", commandName, chatID)
	}
}
func requestAndLog(bot *tgbotapi.BotAPI, c tgbotapi.CallbackConfig, commandName string, chatID int64) {
	if _, err := bot.Request(c); err != nil {
		log.Printf("ПОМИЛКА Request (%s) для %d: %v", commandName, chatID, err)
	} else {
		log.Printf("Надіслано Callback відповідь (%s) для %d", commandName, chatID)
	}
}

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, cfg config.Config) {
	// Обробка CallbackQuery
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		messageID := update.CallbackQuery.Message.MessageID
		userName := update.CallbackQuery.From.UserName
		callbackData := update.CallbackQuery.Data
		log.Printf("Callback від [%s](%d): Data=%s, MsgID=%d", userName, chatID, callbackData, messageID)
		var callbackResponseText string
		originalMessageText := ""
		if update.CallbackQuery.Message != nil {
			originalMessageText = update.CallbackQuery.Message.Text
		}
		switch callbackData {
		case CallbackConfirmCloseGoal:
			log.Printf("Підтверджено закриття цілі для %d", chatID)
			err := DeleteUserGoal(chatID, srv, cfg) // Використовуємо cfg
			if err != nil {
				if strings.Contains(err.Error(), "не знайдено активної цілі") {
					callbackResponseText = "ℹ️ Активну ціль не знайдено."
				} else {
					callbackResponseText = "⚠️ Помилка закриття цілі."
					log.Printf("DeleteUserGoal err: %v", err)
				}
			} else {
				callbackResponseText = "✅ Ціль успішно закрито!"
			}
			editText := tgbotapi.NewEditMessageText(chatID, messageID, originalMessageText+"\n\n"+callbackResponseText)
			editText.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, editText, "confirm_close_goal_edit", chatID)
			keyboard.ShowMainKeyboard(bot, chatID) // Показуємо головну клавіатуру
		case CallbackCancelCloseGoal:
			log.Printf("Скасовано закриття цілі для %d", chatID)
			callbackResponseText = "🚫 Закриття цілі скасовано."
			editText := tgbotapi.NewEditMessageText(chatID, messageID, originalMessageText+"\n\n"+callbackResponseText)
			editText.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, editText, "cancel_close_goal_edit", chatID)
			keyboard.ShowMainKeyboard(bot, chatID) // Показуємо головну клавіатуру
		default:
			log.Printf("Передача Callback '%s' в goal.HandleCallback", callbackData)
			goal.HandleCallback(bot, update.CallbackQuery, srv, cfg) // Передаємо cfg
		}
		answerCallbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, callbackResponseText)
		requestAndLog(bot, answerCallbackCfg, "answer_callback", chatID) // Використовуємо requestAndLog
		return
	}

	// Обробка звичайних повідомлень
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	msgText := update.Message.Text
	userName := update.Message.From.UserName
	log.Printf("[%s] (%d): %s", userName, chatID, msgText)
	currentState := GetUserState(chatID)

	if currentState == StateAwaitingGoalInput {
		HandleGoalInput(bot, update.Message, srv, cfg) // Передаємо cfg
		SetUserState(chatID, StateDefault)             // Скидаємо стан після введення
		keyboard.ShowMainKeyboard(bot, chatID)         // Показуємо головну клавіатуру
		return
	} else if currentState == StateAwaitingInvestmentInput {
		HandleInvestmentInput(bot, update.Message, srv, cfg) // Передаємо cfg
		SetUserState(chatID, StateDefault)                   // Скидаємо стан після введення
		keyboard.ShowMainKeyboard(bot, chatID)               // Показуємо головну клавіатуру
		return
	}

	// Обробка Команд / Кнопок
	switch msgText {
	case "/start", "🔁 Старт":
		commands.StartWork(bot, update.Message, srv, cfg) // Передаємо cfg
		keyboard.ShowMainKeyboard(bot, chatID)
	case "/stop", "⛔️ Стоп":
		commands.StopWork(bot, update.Message, srv, cfg) // Передаємо cfg
		keyboard.ShowMainKeyboard(bot, chatID)
	case "/dayoff", "🏖 Вихідний":
		commands.DayOff(bot, update.Message, srv, cfg) // Передаємо cfg
		keyboard.ShowMainKeyboard(bot, chatID)
	case "/goal", "🎯 Моя ціль":
		log.Printf("Обробка /goal для %d", chatID)
		currentGoal, exists := GetUserGoal(chatID, srv, cfg) // Передаємо cfg
		if exists {
			log.Printf("Знайдено ціль для %d: %+v", chatID, currentGoal)
			// Для monthNameUkrainian, яка визначена в report.go (в тому ж пакеті telegram)
			// ми можемо викликати її напряму.
			goalInfoText := fmt.Sprintf(
				"📌 Ваша поточна ціль:\n\nСума: `%.2f %s`\n(Ціль на %s %d)\nВстановлено: `%s`\n\nЯкщо бажаєте встановити нову ціль, поточна буде автоматично заархівована (статус зміниться на 'Перевизначено').\nЩоб встановити нову, просто введіть суму (напр. `15000 грн`).",
				currentGoal.Amount, currentGoal.Currency,
				monthNameUkrainian(currentGoal.SetDate.In(sheets.KyivLocation).Month()),
				currentGoal.SetDate.In(sheets.KyivLocation).Year(),
				currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006"),
			)
			msg := tgbotapi.NewMessage(chatID, goalInfoText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, msg, "view_goal_exists", chatID)
			goal.HandleMyGoalCommand(bot, chatID)
			SetUserState(chatID, StateAwaitingGoalInput)
			log.Printf("Стан %d -> awaiting_goal (для оновлення існуючої)", chatID)
		} else {
			log.Printf("Активна ціль для %d не знайдена.", chatID)
			goal.HandleMyGoalCommand(bot, chatID)
			SetUserState(chatID, StateAwaitingGoalInput)
			log.Printf("Стан %d -> awaiting_goal", chatID)
		}
	case "/closegoal", "❌ Закрити ціль":
		log.Printf("Обробка /closegoal для %d", chatID)
		currentGoal, exists := GetUserGoal(chatID, srv, cfg) // Передаємо cfg
		if exists {
			confirmationText := fmt.Sprintf(
				"❓ Ви дійсно хочете закрити поточну ціль?\n\nСума: `%.2f %s`\n(Ціль на %s %d)\nВстановлено: `%s`",
				currentGoal.Amount, currentGoal.Currency,
				monthNameUkrainian(currentGoal.SetDate.In(sheets.KyivLocation).Month()),
				currentGoal.SetDate.In(sheets.KyivLocation).Year(),
				currentGoal.SetDate.In(sheets.KyivLocation).Format("02.01.2006"),
			)
			msg := tgbotapi.NewMessage(chatID, confirmationText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			msg.ReplyMarkup = keyboard.CreateConfirmationKeyboard(CallbackConfirmCloseGoal, CallbackCancelCloseGoal)
			sendAndLog(bot, msg, "close_goal_confirm_prompt", chatID)
		} else {
			msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної фінансової цілі для закриття.")
			sendAndLog(bot, msg, "close_goal_no_active", chatID)
			keyboard.ShowMainKeyboard(bot, chatID) 
		}
	case "/add_investment":
		log.Printf("Обробка /add_investment для %d", chatID)
		prompt := "➕ Введіть дані про вашу нову інвестицію у форматі:\n`ТИП, НАЗВА/СИМВОЛ, СУМА ВАЛЮТА, ДАТА (РРРР-ММ-ДД)`\n\nНаприклад:\n`Крипто-холд, BTC, 0.5 BTC, 2023-10-25`\n`Акція, META, 1000 USD, 2024-01-15`"
		msg := tgbotapi.NewMessage(chatID, prompt)
		msg.ParseMode = tgbotapi.ModeMarkdown
		sendAndLog(bot, msg, "add_investment_prompt", chatID)
		SetUserState(chatID, StateAwaitingInvestmentInput)
		log.Printf("Стан %d -> awaiting_investment", chatID)
	case "/funding", "💹 Funding Rates":
		log.Printf("Обробка команди /funding для ChatID: %d", chatID)
		loadingMsg := tgbotapi.NewMessage(chatID, "⏳ Завантажую топ-монети з CoinGecko та ставки з Binance Futures...")
		sentMsgObj, errSendLoad := bot.Send(loadingMsg)
		if errSendLoad != nil {
			log.Printf("ПОМИЛКА send loadingMsg /funding: %v", errSendLoad)
		}
		var fundingReportText string
		topCoins, errCoinGecko := coingecko.GetTopMarketCapCoins(20, "usd")
		if errCoinGecko != nil {
			log.Printf("Помилка CoinGecko API: %v", errCoinGecko)
			fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати топ-монети з CoinGecko: %v", errCoinGecko)
		} else if len(topCoins) == 0 {
			fundingReportText = "Не знайдено топ-монет на CoinGecko."
		} else {
			var targetBinanceSymbols []string
			for _, coin := range topCoins {
				targetBinanceSymbols = append(targetBinanceSymbols, strings.ToUpper(coin.Symbol)+"USDT")
			}
			log.Printf("Сформовано %d цільових символів для Binance: %v", len(targetBinanceSymbols), targetBinanceSymbols)

			allRatesMap, errBinance := binance.GetFundingRates()
			if errBinance != nil {
				log.Printf("Помилка Binance API: %v", errBinance)
				fundingReportText = fmt.Sprintf("⚠️ Не вдалося отримати ставки фінансування з Binance: %v", errBinance)
			} else if len(allRatesMap) == 0 {
				fundingReportText = "Дані про ставки фінансування з Binance недоступні або порожні."
			} else {
				var relevantRatesSlice []binance.FundingInfo
				for _, symbol := range targetBinanceSymbols {
					if rateInfo, ok := allRatesMap[symbol]; ok {
						relevantRatesSlice = append(relevantRatesSlice, rateInfo)
					}
				}
				log.Printf("Знайдено %d релевантних ставок фінансування на Binance.", len(relevantRatesSlice))

				if len(relevantRatesSlice) == 0 {
					fundingReportText = "Не знайдено ставок фінансування для топ-монет на Binance Futures."
				} else {
					sort.SliceStable(relevantRatesSlice, func(i, j int) bool {
						rateI := relevantRatesSlice[i].LastFundingRate
						rateJ := relevantRatesSlice[j].LastFundingRate
						if rateI > 0 && rateJ > 0 {
							return rateI > rateJ
						}
						if rateI < 0 && rateJ < 0 {
							return rateI < rateJ
						}
						return rateI > rateJ
					})

					var sb strings.Builder
					sb.WriteString("📊 **Funding Rates (Binance Futures) для Топ-Монет (CoinGecko):**\n\n")
					sb.WriteString("_Ставки фінансування – це періодичні платежі між трейдерами з Long та Short позиціями, призначені для утримання ціни ф'ючерсного контракту близькою до спотової ціни активу. Розрахунок доходу/витрат на $100 є орієнтовним за один період фінансування і не враховує торгові комісії._\n\n")

					limit := 7
					posCount := 0
					negCount := 0

					sb.WriteString("📈 **Найвищі Позитивні Ставки (Long платить Short):**\n")
					sb.WriteString("_Для цих пар власники Short-позицій отримують ставку від власників Long-позицій._\n")
					sb.WriteString("------------------------------\n")
					foundPos := false
					for _, info := range relevantRatesSlice {
						if posCount >= limit {
							break
						}
						if info.LastFundingRate > 0.0005 {
							profitPer100 := 100 * (info.LastFundingRate / 100.0)
							nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation)
							durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv))
							sb.WriteString(fmt.Sprintf(
								"`%s` (Mark: `$%.2f`)\n"+
									"  Ставка: `+%.4f%%`\n"+
									"  Дохід на $100 Short: `+$%.2f`\n"+
									"  Наступна: `%s` (через %s)\n",
								info.Symbol, info.MarkPrice, info.LastFundingRate, profitPer100,
								nextTimeKyiv.Format("15:04 (02.01)"), durationToNext,
							))
							sb.WriteString("------------------------------\n")
							posCount++
							foundPos = true
						}
					}
					if !foundPos {
						sb.WriteString("_Немає значних позитивних ставок._\n")
					}
					sb.WriteString("\n")

					sb.WriteString("📉 **Найбільш Негативні Ставки (Short платить Long):**\n")
					sb.WriteString("_Для цих пар власники Long-позицій отримують ставку від власників Short-позицій._\n")
					sb.WriteString("------------------------------\n")
					foundNeg := false
					for i := len(relevantRatesSlice) - 1; i >= 0; i-- {
						if negCount >= limit {
							break
						}
						info := relevantRatesSlice[i]
						if info.LastFundingRate < -0.0005 {
							payoutPer100 := 100 * (-info.LastFundingRate / 100.0)
							nextTimeKyiv := info.NextFundingTime.In(sheets.KyivLocation)
							durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv))
							sb.WriteString(fmt.Sprintf(
								"`%s` (Mark: `$%.2f`)\n"+
									"  Ставка: `%.4f%%`\n"+
									"  Дохід на $100 Long: `+$%.2f`\n"+
									"  Наступна: `%s` (через %s)\n",
								info.Symbol, info.MarkPrice, info.LastFundingRate, payoutPer100,
								nextTimeKyiv.Format("15:04 (02.01)"), durationToNext,
							))
							sb.WriteString("------------------------------\n")
							negCount++
							foundNeg = true
						}
					}
					if !foundNeg {
						sb.WriteString("_Немає значних негативних ставок._\n")
					}
					fundingReportText = sb.String()
				}
			}
		}

		if sentMsgObj.MessageID != 0 && errSendLoad == nil {
			editText := tgbotapi.NewEditMessageText(chatID, sentMsgObj.MessageID, fundingReportText)
			editText.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, editText, "funding_report_edit", chatID)
		} else {
			finalMsg := tgbotapi.NewMessage(chatID, fundingReportText)
			finalMsg.ParseMode = tgbotapi.ModeMarkdown
			sendAndLog(bot, finalMsg, "funding_report_new", chatID)
		}
		keyboard.ShowMainKeyboard(bot, chatID)

	case "/motivation":
		motivationText := motivation.GetRandomMotivation()
		msg := tgbotapi.NewMessage(chatID, motivationText)
		sendAndLog(bot, msg, "motivation", chatID)
		keyboard.ShowMainKeyboard(bot, chatID)
	case "/report", "📊 Прогрес":
		ReportProgress(bot, update.Message, srv, cfg)
	default:
		if !strings.HasPrefix(msgText, "/") && currentState == StateDefault {
			log.Printf("Не розпізнаний текстовий ввід від [%s] (%d) поза станом: %s", userName, chatID, msgText)
		} else if strings.HasPrefix(msgText, "/") {
			log.Printf("Не розпізнана команда: [%s] (%d): %s", userName, chatID, msgText)
		}
	}
}

func formatDurationToNextFunding(d time.Duration) string {
	isPast := false
	if d < 0 {
		d = -d
		isPast = true
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours == 0 && minutes == 0 {
		seconds := int(d.Seconds()) % 60
		if isPast {
			return "0с (минув)"
		}
		return fmt.Sprintf("%dс", seconds)
	}
	if isPast {
		return fmt.Sprintf("-%dг %dхв (минув)", hours, minutes)
	}
	return fmt.Sprintf("%dг %dхв", hours, minutes)
}

// Функція monthNameUkrainian ВИДАЛЕНА звідси.
// Вона має бути визначена у файлі report.go того ж пакету telegram
// і буде доступна тут напряму завдяки тому, що вони в одному пакеті.
