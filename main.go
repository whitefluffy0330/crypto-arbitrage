package main

import (
	"context"
	"log"
	"net/http"
	"strings" // Повертаємо імпорт strings

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"

	// tgbotapi тут більше не потрібен напряму
	
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
	// Перевірка, чи створено об'єкт бота
	if bot == nil {
		log.Fatal("Критична помилка: Не вдалося створити об'єкт бота (bot is nil).")
	}
	
	// Обхідна перевірка + безпечне логування імені користувача
	var botUsername string = "[ім'я невідоме]" // Значення за замовчуванням
	// Спочатку перевіряємо bot.Self на nil (що МАЄ працювати), 
	// а потім ID як додаткову перевірку, якщо Self не nil, але порожній.
	if bot.Self == nil || bot.Self.ID == 0 { 
		log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося отримати коректну інформацію про бота (bot.Self is nil or ID is 0). Можливі проблеми з токеном або API Telegram.")
		// Продовжуємо роботу, але ім'я користувача буде невідоме
	} else {
		botUsername = bot.Self.UserName // Присвоюємо ім'я, лише якщо Self та ID виглядають коректно
	}
	log.Printf("Бот @%s ініціалізовано.", botUsername) // Використовуємо безпечну змінну

	// Використовуємо bot (який точно не nil) для отримання токена для шляху вебхука
	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") {
		webhookPath = "/" + webhookPath
		log.Printf("ПОПЕРЕДЖЕННЯ: Додано '/' на початок WEBHOOK_PATH. Використовується шлях: %s", webhookPath)
	}
	// Важливо: Переконайтеся, що ваш WEBHOOK_PATH не містить сам токен! 
	// Краще використовувати секретний рядок, який ви задаєте.
	// Якщо ви все ж хочете додавати токен (не рекомендується), то так:
	// webhookPath = webhookPath + "_" + bot.Token // Цей рядок використовує bot.Token

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

	// Використовуємо безпечну змінну для імені користувача
	log.Printf("Бот @%s готовий до роботи та очікує на оновлення через вебхук...", botUsername) 

	// telegram.StartEveningReport(bot, sheetsService, cfg) 

	telegram.HandleUpdates(updates, bot, sheetsService, cfg) 
}
