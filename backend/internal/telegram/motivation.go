package telegram

import (
	"math/rand"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func SendMotivationalMessage(bot *tgbotapi.BotAPI, chatID int64) {
	phrases := []string{
		"Кожен день — це крок ближче до твоєї мрії! 🚀",
		"Памʼятай, для чого ти почав. 🔥",
		"Сьогоднішня праця — це завтрашня свобода. 💸",
		"Ти будуєш нову реальність прямо зараз! 💪",
		"Навіть маленький крок — це прогрес. 📈",
		"Кожен долар сьогодні — твоя свобода завтра! 🪙",
		"Ти вже не там, де був учора — і це перемога! ✅",
		"Маленькі перемоги — основа великих зрушень! 🧱",
		"Завтрашній успіх народжується в сьогоднішньому зусиллі! 🎯",
		"Ти не просто працюєш — ти інвестуєш у себе! 💼",
	}
	idx := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(len(phrases))
	msg := tgbotapi.NewMessage(chatID, phrases[idx])
	bot.Send(msg)
}
