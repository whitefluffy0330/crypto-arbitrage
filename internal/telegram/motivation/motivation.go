package telegram

import "math/rand"

var motivationalPhrases = []string{
	"🌟 Твоя наполегливість — ключ до успіху!",
	"🚀 Кожен крок наближає тебе до цілі!",
	"🔥 Не зупиняйся, результат вже близько!",
	"💪 Ти справляєшся краще, ніж думаєш!",
	"🎯 Зосередься на головному — ти зможеш усе!",
}

func GetMotivation() string {
	return motivationalPhrases[rand.Intn(len(motivationalPhrases))]
}
