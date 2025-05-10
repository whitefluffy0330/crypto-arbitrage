package main

import (
	"context"
	"log"
	"net/http"
	"strings" 

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"
	
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4"
)

func appContext() context.Context { return context.Background() }

func main() {
	motivation.InitMotivationSeed()
	cfg := config.LoadEnv() 

	bot, err := telegram.InitBot(cfg.BotToken) 
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	// Перевіряємо сам об'єкт бота
	if bot == nil {
		log.Fatal("Критична помилка: Не вдалося створити об'єкт бота (bot is nil).")
	}
	
	// ВИПРАВЛЕНО: Повертаємо обхідну перевірку через bot.Self.ID
	var botUsername string = "[ім'я невідоме]" 
	// Ми припускаємо, що якщо InitBot не повернув помилку, то bot НЕ nil.
	// Тепер перевіряємо, чи було поле Self заповнене, дивлячись на ID.
	if bot.Self.ID == 0 { 
		log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося отримати коректний ID бота (bot.Self.ID is 0). Ім'я користувача буде '[ім'я невідоме]'. Перевірте токен або зв'язок з API Telegram.")
	} else {
		botUsername = bot.Self.UserName 
	}
	log.Printf("Бот @%s ініціалізовано.", botUsername) 

	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") { webhookPath = "/" + webhookPath }

	// WebhookCertPath має бути порожнім, якщо Nginx обробляє TLS
	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, webhookPath, cfg.WebhookCertPath) 
	if err != nil { log.Printf("ПОМИЛКА встановлення вебхука: %v", err) }

	ctx := appContext(); credentials, err := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope) 
	if err != nil { log.Fatalf("Помилка авторизації Google Sheets: %v", err) }
	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil { log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err) }

	updates := bot.ListenForWebhook(webhookPath) 

	go func() {
		log.Printf("Запуск HTTP сервера для вебхука на '%s', шлях: %s", cfg.WebhookListenAddr, webhookPath)
		err_http := http.ListenAndServe(cfg.WebhookListenAddr, nil) // Бот слухає HTTP
		if err_http != nil {
			log.Printf("КРИТИЧНА ПОМИЛКА ЗАПУСКУ HTTP СЕРВЕРА: %v", err_http) 
		}
	}()

	log.Printf("Бот @%s готовий до роботи (слухає на %s, очікує запити від Nginx на %s)...", botUsername, cfg.WebhookListenAddr, webhookPath) 
	telegram.HandleUpdates(updates, bot, sheetsService, cfg) 
}
