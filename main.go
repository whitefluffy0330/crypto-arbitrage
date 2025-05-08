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

func appContext() context.Context {
	return context.Background()
}

func main() {
	motivation.InitMotivationSeed()

	cfg := config.LoadEnv()

	// === Крок ініціалізації бота ===
	log.Println("Спроба ініціалізації бота...")
	bot, err := telegram.InitBot(cfg.BotToken)

	// <<< ДОДАНО ЛОГУВАННЯ >>>
	// Логуємо значення bot та err одразу після виклику InitBot
	log.Printf("Результат telegram.InitBot: bot=%v, err=%v", bot, err)
	// <<< Кінець логування >>>

	// Перевіряємо помилку від InitBot
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	// Перевіряємо сам об'єкт бота
	if bot == nil {
		log.Fatal("Критична помилка: Не вдалося створити об'єкт бота (bot is nil, хоча помилки не було).")
	}
	// Перевіряємо ID бота як обхідний шлях для перевірки bot.Self
	var botUsername string = "[ім'я невідоме]"
	if bot.Self.ID == 0 {
		log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося отримати коректний ID бота (bot.Self.ID is 0). Перевірте токен.")
	} else {
		botUsername = bot.Self.UserName
	}
	log.Printf("Бот @%s ініціалізовано.", botUsername)
	// === Кінець ініціалізації бота ===

	// Налаштування вебхука
	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") { webhookPath = "/" + webhookPath }
	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, webhookPath, cfg.WebhookCertPath)
	if err != nil { log.Printf("ПОМИЛКА встановлення вебхука: %v", err) }

	// Google Sheets
	ctx := appContext()
	credentials, err := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope)
	if err != nil { log.Fatalf("Помилка авторизації Google Sheets: %v", err) }
	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil { log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err) }

	// Запуск слухача вебхуків та HTTPS сервера
	updates := bot.ListenForWebhook(webhookPath)
	go func() {
		log.Printf("Запуск HTTPS сервера для вебхука на '%s', шлях: %s", cfg.WebhookListenAddr, webhookPath)
		err_https := http.ListenAndServeTLS(cfg.WebhookListenAddr, cfg.TLSCertPath, cfg.TLSKeyPath, nil)
		if err_https != nil { log.Printf("КРИТИЧНА ПОМИЛКА ЗАПУСКУ HTTPS СЕРВЕРА: %v", err_https) }
	}()

	log.Printf("Бот @%s готовий до роботи та очікує на оновлення через вебхук...", botUsername)

	// Обробка оновлень
	telegram.HandleUpdates(updates, bot, sheetsService, cfg)
}
