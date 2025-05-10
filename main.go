package main

import (
	"context"
	"log"
	"net/http" // Потрібен для http.ListenAndServe
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
	if err != nil { log.Fatalf("Помилка ініціалізації бота: %v", err) }
	if bot == nil { log.Fatal("Крит. помилка: InitBot повернув nil bot без помилки.")}
	
	var botUsername string = "[ім'я невідоме]"; 
	if bot.Self != nil && bot.Self.ID != 0 { 
		botUsername = bot.Self.UserName 
	} else { 
		log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося отримати інфо про бота (Self or ID is 0).") 
	}
	log.Printf("Бот @%s ініціалізовано.", botUsername) 

	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") { webhookPath = "/" + webhookPath }

	// Для SetWebhook, WebhookCertPath може бути порожнім, якщо Nginx обробляє TLS.
	// Telegram все одно перевірятиме HTTPS доступність WebhookBaseURL + WebhookPath.
	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, webhookPath, cfg.WebhookCertPath) 
	if err != nil { log.Printf("ПОМИЛКА встановлення вебхука: %v", err) }

	ctx := appContext(); credentials, err := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope) 
	if err != nil { log.Fatalf("Помилка авторизації Google Sheets: %v", err) }
	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil { log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err) }

	updates := bot.ListenForWebhook(webhookPath) 

	go func() {
		log.Printf("Запуск HTTP сервера для вебхука на '%s', шлях: %s", cfg.WebhookListenAddr, webhookPath)
		// ЗМІНЕНО: Використовуємо ListenAndServe замість ListenAndServeTLS
		// Шляхи cfg.TLSCertPath та cfg.TLSKeyPath тут більше не потрібні
		err_http := http.ListenAndServe(cfg.WebhookListenAddr, nil) 
		if err_http != nil {
			log.Printf("КРИТИЧНА ПОМИЛКА ЗАПУСКУ HTTP СЕРВЕРА: %v", err_http) 
		}
	}()

	log.Printf("Бот @%s готовий до роботи (слухає на %s, очікує запити від Nginx на %s)...", botUsername, cfg.WebhookListenAddr, webhookPath) 
	telegram.HandleUpdates(updates, bot, sheetsService, cfg) 
}
