package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
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

	if cfg.BotToken == "" {
		log.Fatal("Критична помилка: TELEGRAM_TOKEN не встановлено!")
	}
	if cfg.SpreadsheetID == "" {
		log.Fatal("Критична помилка: SPREADSHEET_ID не встановлено!")
	}

	bot, err := telegram.InitBot(cfg.BotToken)
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	if bot == nil {
		log.Fatal("Критична помилка: Не вдалося створити об'єкт бота (bot is nil).")
	}

	var botUsername string = "[ім'я невідоме]"
	// ПРЯМА ПЕРЕВІРКА ПОЛЯ ID У bot.Self
	// Якщо bot.Self було б nil, це призвело б до паніки в InitBot раніше.
	// Якщо InitBot повернув bot, то bot.Self має бути ініціалізовано (навіть якщо поля порожні).
	// Однак, якщо NewBotAPI з якоїсь причини повернуло bot з bot.Self == nil,
	// а наша оновлена InitBot не відловила це (що малоймовірно з останньою версією InitBot),
	// то ця перевірка все одно може бути проблемою.
	// Але оскільки InitBot тепер має перевірку "if bot.Self == nil",
	// ми можемо припустити, що якщо ми дійшли сюди, то bot.Self *не* nil.
	if bot.Self.ID != 0 { // Якщо ID не нульовий, значить, дані отримані
		botUsername = bot.Self.UserName
	} else {
		// Якщо ID нульовий, але bot.Self не nil (що перевіряється в InitBot)
		log.Printf("ПОПЕРЕДЖЕННЯ: bot.Self.ID == 0. Ім'я користувача буде '[ім'я невідоме]'. UserName з API: '%s'. Перевірте токен.", bot.Self.UserName)
	}
	log.Printf("Бот @%s ініціалізовано.", botUsername)


	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") && webhookPath != "" {
		webhookPath = "/" + webhookPath
	}

	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, webhookPath, cfg.WebhookCertPath)
	if err != nil {
		log.Printf("ПОПЕРЕДЖЕННЯ/ПОМИЛКА встановлення вебхука: %v. Бот продовжить роботу, але вебхук може бути неактивним.", err)
	}

	ctx := appContext()
	credentials, err := google.FindDefaultCredentials(ctx, gsheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Помилка авторизації Google Sheets (FindDefaultCredentials): %v. Перевірте змінну GOOGLE_APPLICATION_CREDENTIALS та доступність файлу credentials.json.", err)
	}

	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err)
	}
	log.Println("Клієнт Google Sheets успішно створено.")

	var updatesChannel tgbotapi.UpdatesChannel
	if webhookPath != "" {
		updatesChannel = bot.ListenForWebhook(webhookPath)
		go func() {
			log.Printf("Запуск HTTP сервера для вебхука на '%s', шлях: %s", cfg.WebhookListenAddr, webhookPath)
			err_http := http.ListenAndServe(cfg.WebhookListenAddr, nil)
			if err_http != nil {
				log.Fatalf("КРИТИЧНА ПОМИЛКА ЗАПУСКУ HTTP СЕРВЕРА для вебхука: %v", err_http)
			}
		}()
		log.Printf("Бот @%s готовий до роботи (слухає на %s, очікує запити від Nginx на %s)...", botUsername, cfg.WebhookListenAddr, webhookPath)
	} else {
		log.Println("ПОПЕРЕДЖЕННЯ: WebhookPath не вказано в конфігурації. Бот не буде слухати вебхуки.")
	}

	if updatesChannel != nil {
		telegram.HandleUpdates(updatesChannel, bot, sheetsService, cfg)
	} else {
		log.Println("Канал оновлень не ініціалізовано. Зупинка роботи (якщо не використовується polling).")
	}
}
