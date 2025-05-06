package motivation

import (
	"math/rand"
	"time" // Потрібен для time.Now().UnixNano()
)

var motivationalPhrases = []string{
	"Ти зможеш досягти всього, якщо продовжиш працювати!",
	"Кожен день — нова можливість наблизитись до мети!",
	"Не зупиняйся! Прогрес — це вже перемога!",
	"Успіх приходить до тих, хто діє!",
	"Ти робиш неймовірну роботу — продовжуй!",
	"Немає нічого неможливого для тебе!",
}

// InitMotivationSeed ініціалізує генератор випадкових чисел.
// Цю функцію потрібно викликати один раз при старті програми (наприклад, з main.go).
func InitMotivationSeed() {
	rand.Seed(time.Now().UnixNano())
}

// GetRandomMotivation повертає випадкову мотиваційну фразу.
// Назва функції змінена з GetMotivationalPhrase для відповідності виклику в handler.go.
func GetRandomMotivation() string {
	if len(motivationalPhrases) == 0 {
		// Запасний варіант, якщо з якихось причин список фраз порожній
		return "Все вийде, головне — не здавайся!"
	}
	return motivationalPhrases[rand.Intn(len(motivationalPhrases))]
}
