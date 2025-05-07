package telegram

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	gsheets "google.golang.org/api/sheets/v4"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" // Це підпакет goal
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
)

// HandleUpdate обробляє вхідні оновлення (повідомлення та callback-запити)
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, spreadsheetID string) {
	// Спочатку обробляємо CallbackQuery, якщо він є
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID // Отримуємо chatID з повідомлення, до якого прив'язаний callback
		log.Printf("Отримано CallbackQuery від [%s] (ChatID: %d), Data: %s", update.CallbackQuery.From.UserName, chatID, update.CallbackQuery.Data)
		
		// Передаємо обробку в підпакет goal
		goal.HandleCallback(bot, update.CallbackQuery)
		return // Завершуємо обробку цього оновлення
	}

	// Якщо це не CallbackQuery, перевіряємо, чи це повідомлення
	if update.Message == nil { // Ігноруємо інші типи оновлень (наприклад, EditedMessage)
		return
	}

	// Отримуємо chatID та текст повідомлення
	chatID := update.Message.Chat.ID
	msgText := update.Message.Text
	userName := update.Message.From.UserName

	log.Printf("[%s] (%d): %s", userName, chatID, msgText)

	// --- Керування станами ---
	currentState := GetUserState(chatID) // Отримуємо поточний стан користувача

	if currentState == StateAwaitingGoalInput {
		// Якщо ми очікували введення цілі, передаємо повідомлення функції HandleGoalInput
		log.Printf("Обробка повідомлення від [%s] як введення цілі (стан: %s)", userName, currentState)
		HandleGoalInput(bot, update.Message) // Викликаємо HandleGoalInput з goal.go (верхнього рівня)
		SetUserState(chatID, StateDefault) // Повертаємо користувача у звичайний стан
		return // Завершуємо обробку
	}
	// --- Кінець керування станами ---

	// Якщо стан звичайний (StateDefault), обробляємо як команду або текст кнопки
	switch msgText {
	case "/start", "🔁 Старт":
		commands.StartWork(bot, update.Message)
	case "/stop", "⛔️ Стоп":
		commands.StopWork(bot, update.Message)
	case "/dayoff", "🏖 Вихідний":
		commands.DayOff(bot, update.Message, srv, spreadsheetID)
	case "/goal", "🎯 Моя ціль":
		// Викликаємо HandleMyGoalCommand з підпакета "goal", щоб надіслати запит
		goal.HandleMyGoalCommand(bot, chatID)
		// Встановлюємо стан очікування відповіді
		SetUserState(chatID, StateAwaitingGoalInput)
	case "/motivation":
		motivationText := motivation.GetRandomMotivation()
		msg := tgbotapi.NewMessage(chatID, motivationText)
		if _, err := bot.Send(msg); err != nil {
			log.Printf("Помилка надсилання мотиваційного повідомлення для чату %d: %v", chatID, err)
		}
	case "/report", "📊 Прогрес":
		ReportProgress(bot, update.Message, srv, spreadsheetID)
	default:
		// Якщо не розпізнано ані команду, ані текст кнопки, показуємо головну клавіатуру
		log.Printf("Не розпізнана команда або текст від [%s]: %s", userName, msgText)
		keyboard.ShowMainKeyboard(bot, chatID)
	}
}
