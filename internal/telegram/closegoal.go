package telegram

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // Використовуємо *sheets.Service
	// gsheets "google.golang.org/api/sheets/v4" // Не потрібен, якщо srv це *sheets.Service
)

// Функція DeleteUserGoal тепер визначена в telegram.go

// CloseUserGoal тепер приймає *sheets.Service та *config.Config
func CloseUserGoal(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv *sheets.Service, cfg *config.Config) {
	chatID := msg.Chat.ID // Беремо chatID з повідомлення
	var msgText string
	err := DeleteUserGoal(chatID, srv, *cfg) // Викликаємо DeleteUserGoal з telegram.go, передаємо *cfg
	if err != nil {
		msgText = fmt.Sprintf("⚠️ Відбулася помилка: %v", err)
		log.Printf("Помилка DeleteUserGoal для ChatID %d: %v", chatID, err)
	} else {
		msgText = "✅ Вашу поточну ціль було позначено як закриту."
		log.Printf("Ціль для ChatID %d успішно закрита.", chatID)
	}
	responseMsg := tgbotapi.NewMessage(chatID, msgText) // Використовуємо responseMsg
	if _, sendErr := bot.Send(responseMsg); sendErr != nil {
		log.Printf("Помилка надсилання повідомлення про закриття цілі для чату %d: %v", chatID, sendErr)
	}
}
