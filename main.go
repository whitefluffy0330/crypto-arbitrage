package main

import (
	"context"
	"log"
	"net/http" // net/http потрібен для ListenAndServeTLS
	"strings"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"

	// tgbotapi не потрібен напряму
	
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4"
)

func appContext() context.Context { return context.Background() }

func main() {
	motivation.InitMotivationSeed()
	cfg := config.LoadEnv() 

	// === Ініціалізація бота (стандартна) ===
	bot, err := telegram.InitBot(cfg.BotToken) 
	// Стандартна перевірка помилки
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	// ВИДАЛЕНО перевірки if bot == nil та if bot.Self.ID == 0. 
	// Покладаємося, що якщо err == nil, то bot та bot.Self валідні.
	// Якщо тут виникне паніка nil pointer - проблема в NewBotAPI.
	log.Printf("Бот @%s ініціалізовано.", bot.Self.UserName) 
	// === Кінець ініціалізації бота ===

	// Налаштування вебхука
	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") { webhookPath = "/" + webhookPath }
	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, webhookPath, cfg.WebhookCertPath) 
	if err != nil { log.Printf("ПОМИЛКА встановлення вебхука: %v", err) }

	// Google Sheets
	ctx := appContext(); credentials, err := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope) 
	if err != nil { log.Fatalf("Помилка авторизації Google Sheets: %v", err) }
	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil { log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err) }

	// Запуск слухача вебхуків та HTTPS сервера
	updates := bot.ListenForWebhook(webhookPath) 
	go func() {
		log.Printf("Запуск HTTPS сервера на '%s', шлях: %s", cfg.WebhookListenAddr, webhookPath)
		err_https := http.ListenAndServeTLS(cfg.WebhookListenAddr, cfg.TLSCertPath, cfg.TLSKeyPath, nil) 
		if err_https != nil { log.Printf("КРИТИЧНА ПОМИЛКА HTTPS СЕРВЕРА: %v", err_https) }
	}()

	log.Printf("Бот @%s готовий до роботи...", bot.Self.UserName) // Використовуємо bot.Self.UserName напряму

	// Обробка оновлень
	telegram.HandleUpdates(updates, bot, sheetsService, cfg) 
}
