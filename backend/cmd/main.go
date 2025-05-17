package main

import (
	"context"
	"log"
	"net/http"
	"strings" // Потрібен для обробки webhookPath

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"         // Ваш пакет для sheets, якщо там є потрібні функції
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation" // Імпорт для ініціалізації

	"golang.org/x/oauth2/google" // Потрібен для Google Sheets Auth
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4" // Аліас для офіційного пакета Sheets
)

func appContext() context.Context { return context.Background() }

func main() {
	motivation.InitMotivationSeed() // Ініціалізація генератора мотиваційних фраз
	cfg := config.LoadEnv()

	// Перевірка обов'язкових змінних (можна розширити)
	if cfg.BotToken == "" {
		log.Fatal("Критична помилка: TELEGRAM_TOKEN не встановлено!")
	}
	if cfg.SpreadsheetID == "" {
		log.Fatal("Критична помилка: SPREADSHEET_ID не встановлено!")
	}
	// GOOGLE_APPLICATION_CREDENTIALS перевіряється при спробі завантаження credentials

	bot, err := telegram.InitBot(cfg.BotToken)
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	if bot == nil { // Додаткова перевірка, хоча InitBot вже має повернути помилку
		log.Fatal("Критична помилка: Не вдалося створити об'єкт бота (bot is nil).")
	}

	var botUsername string = "[ім'я невідоме]"
	if bot.Self != nil && bot.Self.ID != 0 { // Перевірка, чи bot.Self не nil
		botUsername = bot.Self.UserName
	} else {
		log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося отримати коректний ID або bot.Self є nil. Ім'я користувача буде '[ім'я невідоме]'. Перевірте токен або зв'язок з API Telegram.")
	}
	log.Printf("Бот @%s ініціалізовано.", botUsername)

	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") && webhookPath != "" { // Додано перевірку, що webhookPath не порожній
		webhookPath = "/" + webhookPath
	}

	// cfg.WebhookCertPath має бути порожнім, якщо Nginx обробляє TLS.
	// Ми вже налаштували /etc/crypto-bot/environment, щоб TLS_CERT_PATH був порожнім.
	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, webhookPath, cfg.WebhookCertPath)
	if err != nil {
		log.Printf("ПОПЕРЕДЖЕННЯ/ПОМИЛКА встановлення вебхука: %v. Бот продовжить роботу, але вебхук може бути неактивним.", err)
	}

	// Ініціалізація Google Sheets API
	ctx := appContext()
	// FindDefaultCredentials шукає облікові дані в стандартних місцях,
	// включаючи шлях, вказаний у GOOGLE_APPLICATION_CREDENTIALS.
	credentials, err := google.FindDefaultCredentials(ctx, gsheets.SpreadsheetsScope) // Використовуємо gsheets.SpreadsheetsScope
	if err != nil {
		log.Fatalf("Помилка авторизації Google Sheets (FindDefaultCredentials): %v. Перевірте змінну GOOGLE_APPLICATION_CREDENTIALS та доступність файлу credentials.json.", err)
	}

	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials)) // Використовуємо gsheets.NewService
	if err != nil {
		log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err)
	}
	log.Println("Клієнт Google Sheets успішно створено.")

	// Отримуємо оновлення через вебхук
	// webhookPath тут використовується як шлях для HTTP-обробника, який слухає ListenForWebhook
	var updatesChannel tgbotapi.UpdatesChannel
	if webhookPath != "" { // Слухаємо вебхук, тільки якщо шлях для нього вказаний
		updatesChannel = bot.ListenForWebhook(webhookPath)
		// Запускаємо HTTP сервер для вебхука в окремій горутині
		go func() {
			log.Printf("Запуск HTTP сервера для вебхука на '%s', шлях: %s", cfg.WebhookListenAddr, webhookPath)
			// ListenAndServe буде використовувати DefaultServeMux, на якому ListenForWebhook реєструє свій обробник
			err_http := http.ListenAndServe(cfg.WebhookListenAddr, nil)
			if err_http != nil {
				log.Fatalf("КРИТИЧНА ПОМИЛКА ЗАПУСКУ HTTP СЕРВЕРА для вебхука: %v", err_http)
			}
		}()
		log.Printf("Бот @%s готовий до роботи (слухає на %s, очікує запити від Nginx на %s)...", botUsername, cfg.WebhookListenAddr, webhookPath)
	} else {
		log.Println("ПОПЕРЕДЖЕННЯ: WebhookPath не вказано в конфігурації. Бот не буде слухати вебхуки. Якщо ви плануєте використовувати polling, це потрібно реалізувати окремо.")
		// Якщо вебхук не використовується, бот просто завершить роботу, якщо немає іншої логіки (напр. polling).
		// Для polling потрібно було б:
		// u := tgbotapi.NewUpdate(0)
		// u.Timeout = 60
		// updatesChannel = bot.GetUpdatesChan(u)
	}

	// Якщо updatesChannel ініціалізовано (тобто вебхук налаштовано)
	if updatesChannel != nil {
		// Передаємо cfg, який тепер містить AdminChatID
		telegram.HandleUpdates(updatesChannel, bot, sheetsService, cfg) // Передаємо cfg
	} else {
		log.Println("Канал оновлень не ініціалізовано. Зупинка роботи.")
	}
}
