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
	
	// tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5" // Вже імпортується через пакет telegram
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
	// bot.Self вже перевіряється та логується в InitBot
	log.Printf("Бот @%s ініціалізовано (main.go).", bot.Self.UserName)


	webhookPath := cfg.WebhookPath
	if webhookPath == "" {
		log.Println("ПОПЕРЕДЖЕННЯ: WebhookPath не вказано. Спроба роботи в режимі long polling.")
	} else {
		if !strings.HasPrefix(webhookPath, "/") {
			webhookPath = "/" + webhookPath
		}
		if cfg.WebhookBaseURL != "" {
			fullWebhookURL := cfg.WebhookBaseURL + webhookPath
			log.Printf("Повний URL вебхука для встановлення: %s", fullWebhookURL)
			
			webhookConfig, errWebhookCfg := telegram.CreateWebhookConfig(fullWebhookURL, cfg.WebhookCertPath)
			if errWebhookCfg != nil {
				log.Printf("ПОПЕРЕДЖЕННЯ: Помилка створення конфігурації вебхука: %v. Вебхук може бути не встановлено.", errWebhookCfg)
				// Не робимо Fatal, бо бот може спробувати працювати в режимі long polling, якщо URL не вказано
			} else { // Якщо конфігурація створена, встановлюємо вебхук
				err = telegram.SetWebhook(bot, webhookConfig)
				if err != nil {
					log.Printf("ПОПЕРЕДЖЕННЯ/ПОМИЛКА встановлення вебхука: %v. Бот продовжить роботу, але вебхук може бути неактивним або працювати некоректно.", err)
				}
			}
		} else {
			log.Println("ПОПЕРЕДЖЕННЯ: WebhookBaseURL не вказано, хоча WebhookPath є. Вебхук не буде встановлено.")
		}
	}
	

	ctx := appContext()
	var sheetsService *gsheets.Service
	if cfg.GoogleAppCredentialsJSON != "" {
		creds, errCreds := google.CredentialsFromJSON(ctx, []byte(cfg.GoogleAppCredentialsJSON), sheets.SpreadsheetsScope)
		if errCreds == nil {
			sheetsService, err = gsheets.NewService(ctx, option.WithCredentials(creds))
		} else {
			log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося завантажити облікові дані Google з файлу JSON ('%s'): %v. Спроба використати FindDefaultCredentials.", cfg.GoogleAppCredentialsJSON, errCreds)
			defaultCreds, errDefaultCreds := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope)
			if errDefaultCreds != nil {
				log.Fatalf("Помилка авторизації Google Sheets (ні з JSON, ні стандартні): %v", errDefaultCreds)
			}
			sheetsService, err = gsheets.NewService(ctx, option.WithCredentials(defaultCreds))
		}
	} else {
		log.Println("Шлях до GOOGLE_APPLICATION_CREDENTIALS не вказано, спроба використати FindDefaultCredentials для Google Sheets.")
		defaultCreds, errDefaultCreds := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope)
		if errDefaultCreds != nil {
			log.Fatalf("Помилка авторизації Google Sheets (стандартні облікові дані не знайдено): %v", errDefaultCreds)
		}
		sheetsService, err = gsheets.NewService(ctx, option.WithCredentials(defaultCreds))
	}

	if err != nil {
		log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err)
	}
	log.Println("Клієнт Google Sheets успішно створено.")

	var updates tgbotapi.UpdatesChannel
	if cfg.WebhookBaseURL != "" && webhookPath != "" && err == nil { // Перевіряємо, чи не було помилки встановлення вебхука
		updates = bot.ListenForWebhook(webhookPath)
		go func() {
			log.Printf("Запуск HTTP сервера для вебхука на '%s', шлях: %s", cfg.WebhookListenAddr, webhookPath)
			if err_http := http.ListenAndServe(cfg.WebhookListenAddr, nil); err_http != nil {
				log.Fatalf("КРИТИЧНА ПОМИЛКА ЗАПУСКУ HTTP СЕРВЕРА: %v", err_http)
			}
		}()
		log.Printf("Бот @%s готовий до роботи (слухає на %s, очікує запити від Nginx на %s)...", bot.Self.UserName, cfg.WebhookListenAddr, webhookPath)
	} else {
		log.Println("Вебхук не налаштовано/не встановлено належним чином. Запуск в режимі Long Polling.")
		u := tgbotapi.NewUpdate(0)
    	u.Timeout = 60
    	updates = bot.GetUpdatesChan(u)
		log.Printf("Бот @%s готовий до роботи (режим Long Polling)...", bot.Self.UserName)
	}
	
	telegram.HandleUpdates(updates, bot, sheetsService, cfg)
}
