package telegram

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/binance"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/bitget" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/bybit" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/mexc" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/exchanges/okx" 
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // sheets.Service тепер звідси
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" // Функції цілей тепер в telegram.go або goal.go
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	// gsheets "google.golang.org/api/sheets/v4" // srv тепер *sheets.Service
)

const (
	CallbackConfirmCloseGoal = "confirm_close_goal" 
	CallbackCancelCloseGoal  = "cancel_close_goal"
	MaxTelegramMessageSize   = 4096 
)

func sendAndLog(bot *tgbotapi.BotAPI, c tgbotapi.Chattable, commandName string, chatID int64) {
	if _, err := bot.Send(c); err != nil {
		log.Printf("ПОМИЛКА надсилання (%s) для %d: %v", commandName, chatID, err)
	}
}
func requestAndLog(bot *tgbotapi.BotAPI, c tgbotapi.CallbackConfig, commandName string, chatID int64) {
	if _, err := bot.Request(c); err != nil {
		log.Printf("ПОМИЛКА Request (%s) для %d: %v", commandName, chatID, err)
	}
}

func handleFundingExchangeSelection(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, chatID int64, exchangeName, exchangeCallbackPrefix string) {
	log.Printf("Обробка запиту фандингу для біржі: %s (ChatID: %d)", exchangeName, chatID)
	answerCallback := tgbotapi.NewCallback(query.ID, fmt.Sprintf("Завантажую ставки з %s...", exchangeName))
	requestAndLog(bot, answerCallback, "funding_exchange_ack", chatID)

	loadingMsgText := fmt.Sprintf("⏳ Завантажую ставки з %s...", exchangeName)
	var originalMessageID int 
	if query.Message != nil {
		originalMessageID = query.Message.MessageID
		editMsg := tgbotapi.NewEditMessageText(chatID, originalMessageID, loadingMsgText)
		editMsg.ReplyMarkup = nil 
		if _, err := bot.Send(editMsg); err != nil {
			log.Printf("Помилка редагування повідомлення (ID: %d) ... для %s: %v. Спроба нового.", originalMessageID, exchangeName, err)
			newMsg := tgbotapi.NewMessage(chatID, loadingMsgText)
			sentLoadingMsg, errSend := bot.Send(newMsg)
			if errSend != nil { log.Printf("Помилка надсилання нового 'Завантажую...' для %s: %v", exchangeName, errSend); /* ... */ return }
			originalMessageID = sentLoadingMsg.MessageID 
		}
	} else { /* ... (код для query.Message == nil) ... */ }

	var rates []exchanges.UnifiedFundingRateInfo; var err error
	switch exchangeCallbackPrefix {
	case keyboard.CallbackFundingBinance: rates, err = binance.GetFundingRates()
	// ... (інші біржі) ...
	default: /* ... */ keyboard.ShowMainKeyboard(bot, chatID); return
	}

	currentFundingThreshold := GetUserFundingThreshold(chatID) // З telegram.go
	var reportText string
	if err != nil { reportText = fmt.Sprintf("⚠️ %s: не вдалося завантажити: %v", exchangeName, err)
	} else if len(rates) == 0 { reportText = fmt.Sprintf("ℹ️ %s: дані порожні (поріг: `%.4f%%`).", exchangeName, currentFundingThreshold*100)
	} else {
		// ... (логіка форматування звіту Funding Rates, використовуйте formatDurationToNextFunding з цього файлу) ...
		// Приклад: durationToNext := formatDurationToNextFunding(time.Until(nextTimeKyiv))
		//          sb.WriteString(fmt.Sprintf("... Наступна: %s\n", nextFundingDisplay));
		reportText = "Приклад звіту Funding Rates" // ЗАМІНІТЬ НА ВАШУ ЛОГІКУ ФОРМАТУВАННЯ
	}
	// ... (код надсилання звіту) ...
	keyboard.ShowMainKeyboard(bot, chatID)
}

// HandleUpdate - основний обробник оновлень
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *sheets.Service, cfg *config.Config) { // Змінено тип srv
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		// ... (решта вашої логіки обробки CallbackQuery з відповіді #64) ...
		// Переконайтеся, що виклики DeleteUserGoal, GetUserGoal використовують функції з telegram.go
		// Приклад для CallbackConfirmCloseGoal:
		// err := DeleteUserGoal(chatID, srv, *cfg) // DeleteUserGoal з telegram.go
		// ...
		// Якщо у вас є goal.HandleCallback, переконайтеся, що він існує і правильно викликається
		// goal.HandleCallback(bot, update.CallbackQuery, srv, cfg) // Або просто HandleGoalCallback, якщо в одному пакеті
		return
	}

	if update.Message == nil { return }
	chatID := update.Message.Chat.ID; msgText := update.Message.Text; userName := update.Message.From.UserName
	log.Printf("[%s] (%d): %s", userName, chatID, msgText)
	currentState := GetUserState(chatID) // З telegram.go
	// ... (решта вашої логіки switch currentState та switch msgText з відповіді #64) ...
	// Переконайтеся, що виклики функцій (наприклад, HandleGoalInput, ReportProgress, CloseUserGoal, HandleSpreadsCommand)
	// відповідають їх визначенням в інших файлах цього пакету.
	// Наприклад, HandleGoalInput(bot, update.Message, srv, *cfg)
	// ReportProgress(bot, update.Message, srv, *cfg)
	// CloseUserGoal(bot, chatID, srv, *cfg) // CloseUserGoal приймає chatID
} 

// formatDurationToNextFunding - ця функція має бути визначена тільки тут
func formatDurationToNextFunding(d time.Duration) string {
	isPast := false; if d < 0 { d = -d; isPast = true }
	hours := int(d.Hours()); minutes := int(d.Minutes()) % 60
	if hours == 0 && minutes == 0 {
		seconds := int(d.Seconds()) % 60
		if isPast { return "0с (минув)" }
		return fmt.Sprintf("%dс", seconds)
	}
	if isPast { return fmt.Sprintf("-%dг %dхв (минув)", hours, minutes) }
	return fmt.Sprintf("%dг %dхв", hours, minutes)
}
