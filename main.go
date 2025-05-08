package main

import (
	"context"
	// "fmt" // ВИДАЛЕНО НЕПОТРІБНИЙ ІМПОРТ
	"log"
	"net/http"
	"strings" 

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"

	// tgbotapi тут більше не потрібен
	
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4"
)

func appContext() context.Context {
	return context.Background()
}

func main() {
	motivation.InitMotivationSeed()
	// Діагностичний рядок var _ tgbotapi.Update видалено

	cfg := config.LoadEnv() 

	bot, err := telegram.InitBot(cfg.BotToken) 
	// Перевіряємо помилку від InitBot
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	// ВИПРАВЛЕНО: Перевіряємо bot на nil ОКРЕМО
	if bot == nil {
		log.Fatal("Критична помилка: Не вдалося створити об'єкт бота (bot is nil) після ініціалізації.")
	}
	// Тільки якщо bot не nil, перевіряємо bot.Self
	if bot.Self == nil { 
		log.Fatal("Критична помилка: Не вдалося отримати інформацію про бота (bot.Self is nil) після ініціалізації.")
	}
	// Тепер безпечно використовувати bot.Self.UserName
	log.Printf("Бот @%s ініціалізовано.", bot.Self.UserName) 

	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") {
		webhookPath = "/" + webhookPath
		log.Printf("ПОПЕРЕДЖЕННЯ: Додано '/' на початок WEBHOOK_PATH. Використовується шлях: %s", webhookPath)
	}

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
			// Можливо, тут варто теж викликати log.Fatal або інший механізм зупинки,
			// оскільки без HTTPS сервера вебхук не працюватиме. Але поки залишимо Printf.
		}
	}()

	log.Printf("Бот @%s готовий до роботи та очікує на оновлення через вебхук...", bot.Self.UserName) 

	// telegram.StartEveningReport(bot, sheetsService, cfg) 

	telegram.HandleUpdates(updates, bot, sheetsService, cfg) 
}
