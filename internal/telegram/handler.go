package telegram

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	gsheets "google.golang.org/api/sheets/v4"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/commands"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal" // Це підпакет goal
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/keyboard"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
)

func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, srv *gsheets.Service, spreadsheetID string) {
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		log.Printf("Отримано CallbackQuery від [%s] (ChatID: %d), Data: %s", update.CallbackQuery.From.UserName, chatID, update.CallbackQuery.Data)
		
		goal.HandleCallback(bot, update.CallbackQuery) 
		callbackConfig := tgbotapi.NewCallback(update.CallbackQuery.ID, "") 
		if _, err := bot.Request(callbackConfig); err != nil {
			log.Printf("Помилка відповіді на CallbackQuery: %v", err)
		}
		return
	}

	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	msgText := update.Message.Text
	userName := update.Message.From.UserName

	log.Printf("[%s] (%d): %s", userName, chatID, msgText)

	currentState := GetUserState(chatID)

	if currentState == StateAwaitingGoalInput {
		// Перевіряємо, чи введене повідомлення не є однією з команд/кнопок
		isCommandOrButton := false
		switch msgText {
		case "/start", "🔁 Старт", "/stop", "⛔️ Стоп", "/dayoff", "🏖 Вихідний", "/goal", "🎯 Моя ціль", "/closegoal", "❌ Закрити ціль", "/motivation", "/report", "📊 Прогрес":
			isCommandOrButton = true
		}

		if isCommandOrButton {
			log.Printf("Користувач %s (ChatID: %d) був у стані StateAwaitingGoalInput, але надіслав команду/натиснув кнопку '%s'. Скидаємо стан.", userName, chatID, msgText)
			SetUserState(chatID, StateDefault) 
			// Дозволяємо обробити команду/кнопку в switch нижче
		} else {
			// Якщо це не команда/кнопка, обробляємо як введення цілі
			log.Printf("Обробка повідомлення від [%s] як введення цілі (стан: %s)", userName, currentState)
			HandleGoalInput(bot, update.Message) 
			SetUserState(chatID, StateDefault)
			// Після введення цілі покажемо головну клавіатуру
			keyboard.ShowMainKeyboard(bot, chatID)
			return 
		}
		// Оновлюємо currentState, оскільки він міг змінитися
		currentState = GetUserState(chatID) 
	}

	switch msgText {
	case "/start", "🔁 Старт":
		commands.StartWork(bot, update.Message)
		keyboard.ShowMainKeyboard(bot, chatID) // Показуємо клавіатуру після дії
	case "/stop", "⛔️ Стоп":
		commands.StopWork(bot, update.Message)
		keyboard.ShowMainKeyboard(bot, chatID) // Показуємо клавіатуру після дії
	case "/dayoff", "🏖 Вихідний":
		commands.DayOff(bot, update.Message, srv, spreadsheetID)
		keyboard.ShowMainKeyboard(bot, chatID) // Показуємо клавіатуру після дії
	case "/goal", "🎯 Моя ціль":
		log.Printf("Обробка команди /goal для ChatID: %d", chatID)
		currentGoal, exists := GetUserGoal(chatID) 
		if exists {
			log.Printf("Для ChatID %d знайдено існуючу ціль: %+v", chatID, currentGoal)
			goalInfoText := fmt.Sprintf(
				"📌 Ваша поточна фінансова ціль:\n\n"+
					"Сума: %.2f %s\n"+
					"Термін: %d днів\n"+
					"Встановлено: %s\n\n"+
					"Щоб встановити нову ціль (вона перезапише поточну), натисніть цю кнопку ще раз або введіть /goal.",
				currentGoal.Amount, currentGoal.Currency, currentGoal.Days, currentGoal.SetDate.Format("02.01.2006"),
			)
			msg := tgbotapi.NewMessage(chatID, goalInfoText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Помилка надсилання інформації про поточну ціль для ChatID %d: %v", chatID, err)
			} else {
				log.Printf("Повідомлення про поточну ціль надіслано для ChatID %d", chatID)
			}
			// Після показу поточної цілі, показуємо головну клавіатуру і не переходимо в стан очікування
			keyboard.ShowMainKeyboard(bot, chatID)
		} else {
			log.Printf("Для ChatID %d активна ціль не знайдена. Пропонуємо встановити.", chatID)
			// Якщо цілі немає, пропонуємо встановити
			goal.HandleMyGoalCommand(bot, chatID) // Функція з підпакета goal, яка надсилає запит "Надішли свою ціль..."
			SetUserState(chatID, StateAwaitingGoalInput)
			log.Printf("Стан для ChatID %d встановлено в StateAwaitingGoalInput після /goal", chatID)
			// Клавіатура тут не потрібна, бо користувач має ввести текст
		}

	case "/closegoal", "❌ Закрити ціль": 
		log.Printf("Обробка команди /closegoal для ChatID: %d", chatID)
		_, exists := GetUserGoal(chatID)
		if exists {
			log.Printf("Закриття існуючої цілі для ChatID %d", chatID)
			CloseUserGoal(bot, chatID) 
		} else {
			log.Printf("Немає активної цілі для закриття для ChatID %d", chatID)
			msg := tgbotapi.NewMessage(chatID, "ℹ️ У вас немає активної цілі для закриття.")
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Помилка надсилання повідомлення 'немає цілі для закриття' для ChatID %d: %v", chatID, err)
			}
		}
		keyboard.ShowMainKeyboard(bot, chatID) // Показуємо клавіатуру після дії

	case "/motivation":
		motivationText := motivation.GetRandomMotivation()
		msg := tgbotapi.NewMessage(chatID, motivationText)
		if _, err := bot.Send(msg); err != nil {
			log.Printf("Помилка надсилання мотиваційного повідомлення для чату %d: %v", chatID, err)
		}
		keyboard.ShowMainKeyboard(bot, chatID) // Показуємо клавіатуру після дії
	case "/report", "📊 Прогрес":
		ReportProgress(bot, update.Message, srv, spreadsheetID)
		keyboard.ShowMainKeyboard(bot, chatID) // Показуємо клавіатуру після дії
	default:
		log.Printf("Не розпізнана команда або текст від [%s]: %s. Показано головну клавіатуру.", userName, msgText)
		keyboard.ShowMainKeyboard(bot, chatID)
	}
}
