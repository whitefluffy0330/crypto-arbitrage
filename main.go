package main

import (
	"context"
	"log"
	"net/http"
	"strings" // Додано для перевірки префікса шляху вебхука

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"

	// Імпорт tgbotapi більше не потрібен напряму в main.go, оскільки типи використовуються через пакет telegram
	// tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5" 
	
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4"
)

func appContext() context.Context {
	return context.Background()
}

func main() {
	motivation.InitMotivationSeed()

	// Завантажуємо всю конфігурацію
	cfg := config.LoadEnv() 
	// Перевірка наявності обов'язкових полів вже відбувається всередині LoadEnv()

	// Ініціалізація бота
	bot, err := telegram.InitBot(cfg.BotToken)
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	log.Printf("Бот @%s ініціалізовано.", bot.Self.UserName)

	// Перевіряємо та форматуємо шлях вебхука
	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") {
		webhookPath = "/" + webhookPath
		log.Printf("ПОПЕРЕДЖЕННЯ: Додано '/' на початок WEBHOOK_PATH. Використовується шлях: %s", webhookPath)
	}

	// Встановлення Webhook з використанням параметрів з cfg
	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, webhookPath, cfg.WebhookCertPath) // Використовуємо значення з cfg
	if err != nil {
		log.Fatalf("Помилка встановлення вебхука: %v", err)
	}

	// Ініціалізація Google Sheets
	ctx := appContext()
	credentials, err := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope) 
	if err != nil {
		log.Fatalf("Помилка авторизації Google Sheets (перевірте GOOGLE_APPLICATION_CREDENTIALS): %v", err)
	}
	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err)
	}

	// Отримання каналу оновлень
	updates := bot.ListenForWebhook(webhookPath) // Слухаємо на тому ж шляху, що й встановлювали

	// Запуск HTTPS сервера в окремій горутині
	go func() {
		log.Printf("Запуск HTTPS сервера для вебхука на '%s', шлях: %s", cfg.WebhookListenAddr, webhookPath)
		// Використовуємо шляхи до сертифікатів з cfg
		err_https := http.ListenAndServeTLS(cfg.WebhookListenAddr, cfg.TLSCertPath, cfg.TLSKeyPath, nil) 
		if err_https != nil {
			log.Fatalf("Помилка запуску HTTPS сервера: %v", err_https)
		}
	}()

	log.Printf("Бот @%s готовий до роботи та очікує на оновлення через вебхук...", bot.Self.UserName)

	// telegram.StartEveningReport(bot, sheetsService, cfg) // Функція вечірнього звіту (якщо є)

	// Запуск головного обробника оновлень, передаємо всю конфігурацію cfg
	telegram.HandleUpdates(updates, bot, sheetsService, cfg)
}
