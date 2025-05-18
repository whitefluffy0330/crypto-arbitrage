package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings" // Додано для роботи зі шляхом вебхука

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5" // Додано, якщо InitBot його не імпортує напряму
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4"
)

func appContext() context.Context { return context.Background() }

func main() {
	// Ініціалізація генератора випадкових чисел для мотиваційних фраз
	motivation.InitMotivationSeed()

	// Завантаження конфігурації
	cfg := config.LoadEnv()

	// Ініціалізація бота
	bot, err := telegram.InitBot(cfg.BotToken)
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	if bot == nil {
		log.Fatal("Критична помилка: Не вдалося створити об'єкт бота (bot is nil).")
	}

	var botUsername string = "[ім'я невідоме]"
	// Перевіряємо bot.Self після успішної ініціалізації
	if bot.Self.ID != 0 { // Порівняння з 0, а не з nil
		botUsername = bot.Self.UserName
		log.Printf("Бот успішно ініціалізований: ID=%d, UserName='%s'", bot.Self.ID, botUsername)
	} else {
		// Спробуємо отримати інформацію про бота ще раз, якщо bot.Self.ID порожній
		userInfo, errGetMe := bot.GetMe()
		if errGetMe == nil && userInfo.ID != 0 {
			botUsername = userInfo.UserName
			bot.Self = *userInfo // Оновлюємо інформацію про бота
			log.Printf("Інформацію про бота отримано повторним запитом: @%s (ID: %d)", botUsername, bot.Self.ID)
		} else {
			log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося отримати коректні дані бота (bot.Self.ID is 0, помилка GetMe: %v). Ім'я користувача буде '[ім'я невідоме]'. Перевірте токен або зв'язок з API Telegram.", errGetMe)
		}
	}
	log.Printf("Бот @%s ініціалізовано (після перевірок).", botUsername)


	// Формування повного URL для вебхука та шляху для слухача
	webhookPath := cfg.WebhookPath
	if webhookPath == "" {
		log.Fatal("Критична помилка: WebhookPath не вказано в конфігурації.")
	}
	if !strings.HasPrefix(webhookPath, "/") {
		webhookPath = "/" + webhookPath
	}
	
	var fullWebhookURL string
	if cfg.WebhookBaseURL != "" {
		fullWebhookURL = cfg.WebhookBaseURL + webhookPath
		log.Printf("Повний URL вебхука: %s", fullWebhookURL)

		// Встановлення вебхука
		// cfg.WebhookCertPath має бути порожнім, якщо Nginx обробляє TLS
		webhookConfig, errWebhookCfg := telegram.CreateWebhookConfig(fullWebhookURL, cfg.WebhookCertPath) // ВИПРАВЛЕНО
		if errWebhookCfg != nil {
			log.Fatalf("Помилка створення конфігурації вебхука: %v", errWebhookCfg)
		}

		err = telegram.SetWebhook(bot, webhookConfig) // ВИПРАВЛЕНО
		if err != nil {
			log.Printf("ПОПЕРЕДЖЕННЯ/ПОМИЛКА встановлення вебхука: %v. Бот продовжить роботу, але вебхук може бути неактивним або працювати некоректно.", err)
		}
	} else {
		log.Println("ПОПЕРЕДЖЕННЯ: WebhookBaseURL не вказано, вебхук не буде встановлено. Бот працюватиме в режимі long polling (якщо updates налаштовано).")
	}


	// Ініціалізація Google Sheets сервісу
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

	// Отримуємо оновлення через вебхук
	// bot.ListenForWebhook повертає канал, тому його потрібно присвоїти
	var updates tgbotapi.UpdatesChannel
	if cfg.WebhookBaseURL != "" { // Налаштовуємо слухача вебхука тільки якщо URL вебхука встановлено
		updates = bot.ListenForWebhook(webhookPath) // Шлях має бути відносним, наприклад /SECRET_PATH
		// Запускаємо HTTP сервер для слухання запитів від Nginx
		go func() {
			log.Printf("Запуск HTTP сервера для вебхука на '%s', шлях: %s", cfg.WebhookListenAddr, webhookPath)
			if err_http := http.ListenAndServe(cfg.WebhookListenAddr, nil); err_http != nil {
				log.Fatalf("КРИТИЧНА ПОМИЛКА ЗАПУСКУ HTTP СЕРВЕРА: %v", err_http)
			}
		}()
		log.Printf("Бот @%s готовий до роботи (слухає на %s, очікує запити від Nginx на %s)...", botUsername, cfg.WebhookListenAddr, webhookPath)

	} else { // Якщо вебхук не встановлено, використовуємо long polling
		log.Println("Вебхук не встановлено, спроба отримати оновлення через GetUpdatesChan...")
		u := tgbotapi.NewUpdate(0)
    	u.Timeout = 60
    	updates = bot.GetUpdatesChan(u)
	}


	telegram.HandleUpdates(updates, bot, sheetsService, cfg)
}
