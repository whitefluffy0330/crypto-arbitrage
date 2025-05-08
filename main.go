package main

import (
	"context"
	"fmt" // Додаємо fmt для помилки
	"log"
	"net/http"
	"strings"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"

	// tgbotapi більше не потрібен напряму
	
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4"
)

func appContext() context.Context {
	return context.Background()
}

func main() {
	motivation.InitMotivationSeed()

	// var _ tgbotapi.Update // Цей рядок більше не потрібен, видаляємо його

	cfg := config.LoadEnv() 

	bot, err := telegram.InitBot(cfg.BotToken)
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	
	// <<< ПОКРАЩЕННЯ: Додаткова перевірка bot.Self >>>
	if bot == nil || bot.Self == nil {
		// Ця ситуація не повинна виникати, якщо InitBot не повернув помилку,
		// але додаємо перевірку про всяк випадок.
		log.Fatal("Критична помилка: Не вдалося отримати інформацію про бота (bot.Self is nil) після ініціалізації.")
	}
	// <<< Кінець покращення >>>

	log.Printf("Бот @%s ініціалізовано.", bot.Self.UserName) // Тепер ця лінія безпечна

	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") {
		webhookPath = "/" + webhookPath
		log.Printf("ПОПЕРЕДЖЕННЯ: Додано '/' на початок WEBHOOK_PATH. Використовується шлях: %s", webhookPath)
	}

	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, webhookPath, cfg.WebhookCertPath) 
	if err != nil {
		// Не робимо Fatal, можливо вебхук вже встановлено або є тимчасова проблема
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
			// Використовуємо log.Printf замість log.Fatalf, щоб не зупиняти основний потік обробки оновлень, якщо він ще працює
			log.Printf("КРИТИЧНА ПОМИЛКА ЗАПУСКУ HTTPS СЕРВЕРА: %v", err_https) 
		}
	}()

	log.Printf("Бот @%s готовий до роботи та очікує на оновлення через вебхук...", bot.Self.UserName) 

	// telegram.StartEveningReport(bot, sheetsService, cfg) 

	telegram.HandleUpdates(updates, bot, sheetsService, cfg) 
}
