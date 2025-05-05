package motivation

import (
	"math/rand"
	"time"
)

var motivationalPhrases = []string{
	"Ти зможеш досягти всього, якщо продовжиш працювати!",
	"Кожен день — нова можливість наблизитись до мети!",
	"Не зупиняйся! Прогрес — це вже перемога!",
	"Успіх приходить до тих, хто діє!",
	"Ти робиш неймовірну роботу — продовжуй!",
	"Немає нічого неможливого для тебе!",
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func GetMotivationalPhrase() string {
	return motivationalPhrases[rand.Intn(len(motivationalPhrases))]
}
