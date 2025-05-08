package main

import (
	"context"
	// "fmt" // Видалено
	"log"
	"net/http"
	"strings" // Потрібен для перевірки шляху вебхука

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"

	// tgbotapi не потрібен напряму
	
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4"
)

func appContext() context.Context {
	return context.Background()
}

func main() {
	motivation.InitMotivationSeed()

	cfg := config.LoadEnv() 

	bot, err := telegram.InitBot(cfg.BotToken) 
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	if bot == nil { // Перевіряємо сам об'єкт бота
		log.Fatal("Критична помилка: Не вдалося створити об'єкт бота (bot is nil).")
	}
	
	// ОБХІДНА ПЕРЕВІРКА: Замість bot.Self == nil, перевіряємо bot.Self.ID == 0
	// Це ризиковано, якщо bot.Self дійсно nil, але спробуємо задовольнити компілятор.
	var botUsername string = "[ім'я невідоме]" // Значення за замовчуванням
	if bot.Self.ID == 0 { 
		log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося отримати коректний ID бота (bot.Self.ID is 0). Ім'я користувача може бути невірним. Перевірте токен або зв'язок з API Telegram.")
		// Не завершуємо роботу, але ім'я буде невідомим
	} else {
		// Якщо ID не нульовий, припускаємо, що можна отримати UserName
		botUsername = bot.Self.UserName 
	}
	log.Printf("Бот @%s ініціалізовано.", botUsername) // Використовуємо безпечну змінну

	// Формуємо шлях для вебхука
	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") {
		webhookPath = "/" + webhookPath
		log.Printf("ПОПЕРЕДЖЕННЯ: Додано '/' на початок WEBHOOK_PATH. Використовується шлях: %s", webhookPath)
	}
	// Примітка: Переконайтеся, що cfg.WebhookPath містить ваш секретний шлях.
	// НЕ використовуйте тут bot.Token для формування шляху з міркувань безпеки.

	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, webhookPath, cfg.WebhookCertPath) 
	if err != nil {
		log.Printf("ПОМИЛКА встановлення вебхука (продовжуємо роботу): %v", err)
	}

	ctx := appContext()
	credentials, err := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope) 
	if err != nil {
		log.Fatalf("Помилка авторизації Google Sheets: %v", err)
	}
	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err)
	}

	updates := bot.ListenForWebhook(webhookPath) 

	go func() {
		log.Printf("Запуск HTTPS сервера для вебхука на '%s', шлях: %s", cfg.WebhookListenAddr, webhookPath)
		err_https := http.ListenAndServeTLS(cfg.WebhookListenAddr, cfg.TLSCertPath, cfg.TLSKeyPath, nil) 
		if err_https != nil {
			log.Printf("КРИТИЧНА ПОМИЛКА ЗАПУСКУ HTTPS СЕРВЕРА: %v", err_https) 
		}
	}()

	log.Printf("Бот @%s готовий до роботи та очікує на оновлення через вебхук...", botUsername) 

	// telegram.StartEveningReport(bot, sheetsService, cfg) 

	telegram.HandleUpdates(updates, bot, sheetsService, cfg) 
}
