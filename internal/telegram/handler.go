package telegram

import (
	"log" // Для логування дій та помилок

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// Використовуємо аліас gsheets для google.golang.org/api/sheets/v4, щоб було зрозуміло,
	// що srv - це сервіс Google API.
	gsheets "google.golang.org/api/sheets/v4"

	// Імпорти ваших внутрішніх пакетів
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" // Це підпакет goal
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	// Пакет config тут не потрібен, оскільки spreadsheetID передається напряму.
	// Пакет internal/sheets тут також не потрібен напряму, оскільки srv має тип gsheets.Service.
)

// HandleUpdate обробляє вхідні повідомлення та callback-запити.
// spreadsheetID передається напряму, а не через config.Config.
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, spreadsheetID string) {
	if update.Message != nil { // Якщо це повідомлення
		log.Printf("[%s] (%d): %s", update.Message.From.UserName, update.Message.Chat.ID, update.Message.Text)

		// Обробка команд та текстів з кнопок
		switch update.Message.Text {
		case "/start", "🔁 Старт":
			commands.StartWork(bot, update.Message)
		case "/stop", "⛔️ Стоп":
			commands.StopWork(bot, update.Message)
		case "/dayoff", "🏖 Вихідний":
			commands.DayOff(bot, update.Message, srv, spreadsheetID)
		case "/goal", "🎯 Моя ціль":
			// Викликаємо HandleMyGoalCommand з підпакета "goal"
			goal.HandleMyGoalCommand(bot, update.Message.Chat.ID)
		case "/motivation":
			// Викликаємо GetRandomMotivation з пакета "motivation"
			motivationText := motivation.GetRandomMotivation()
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, motivationText)
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Помилка надсилання мотиваційного повідомлення для чату %d: %v", update.Message.Chat.ID, err)
			}
		case "/report", "📊 Прогрес":
			// Викликаємо ReportProgress з поточного пакета "telegram" (файл report.go)
			ReportProgress(bot, update.Message, srv, spreadsheetID)
		default:
			// Якщо команда або текст кнопки не розпізнано, показуємо головну клавіатуру
			log.Printf("Не розпізнана команда або текст від [%s]: %s", update.Message.From.UserName, update.Message.Text)
			keyboard.ShowMainKeyboard(bot, update.Message.Chat.ID)
		}
	} else if update.CallbackQuery != nil { // Якщо це callback-запит від inline-кнопки
		log.Printf("Отримано CallbackQuery від [%s], Data: %s", update.CallbackQuery.From.UserName, update.CallbackQuery.Data)
		// Викликаємо HandleCallback з підпакета "goal"
		goal.HandleCallback(bot, update.CallbackQuery)

		// Не забувайте відповідати на CallbackQuery, щоб прибрати "годинник" на кнопці
		// callbackResp := tgbotapi.NewCallback(update.CallbackQuery.ID, update.CallbackQuery.Data)
		// if _, err := bot.AnswerCallbackQuery(callbackResp); err != nil {
		// 	log.Printf("Помилка відповіді на CallbackQuery: %v", err)
		// }
	}
	// Тут можна додати обробку інших типів оновлень, якщо потрібно
	// (наприклад, update.EditedMessage, update.ChannelPost тощо)
}
