package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation" // ДОДАНО ІМПОРТ

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4"
)

func appContext() context.Context { return context.Background() }

func main() {
	motivation.InitMotivationSeed() // <--- ДОДАНО ІНІЦІАЛІЗАЦІЮ МОТИВАЦІЇ
	cfg := config.LoadEnv()

	bot, err := telegram.InitBot(cfg.BotToken)
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	if bot == nil {
		log.Fatal("Критична помилка: Не вдалося створити об'єкт бота (bot is nil).")
	}

	var botUsername string = "[ім'я невідоме]"
	if bot.Self.ID == 0 {
		log.Printf("ПОПЕРЕДЖЕННЯ: Не вдалося отримати коректний ID бота (bot.Self.ID is 0). Ім'я користувача буде '[ім'я невідоме]'. Перевірте токен або зв'язок з API Telegram.")
	} else {
		botUsername = bot.Self.UserName
	}
	log.Printf("Бот @%s ініціалізовано.", botUsername)

	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") {
		webhookPath = "/" + webhookPath
	}

	// WebhookCertPath має бути порожнім, якщо Nginx обробляє TLS
	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, webhookPath, cfg.WebhookCertPath)
	if err != nil {
		// Не робимо Fatal, оскільки бот може працювати в режимі polling або вебхук вже встановлено
		log.Printf("ПОПЕРЕДЖЕННЯ/ПОМИЛКА встановлення вебхука: %v. Бот продовжить роботу, але вебхук може бути неактивним.", err)
	}

	// Ініціалізація Google Sheets API
	// Використовуємо GOOGLE_APPLICATION_CREDENTIALS, який має бути встановлений у середовищі
	// і вказувати на ваш credentials.json файл.
	ctx := appContext()
	// FindDefaultCredentials шукає облікові дані в стандартних місцях,
	// включаючи шлях, вказаний у GOOGLE_APPLICATION_CREDENTIALS.
	credentials, err := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Помилка авторизації Google Sheets (FindDefaultCredentials): %v. Перевірте змінну GOOGLE_APPLICATION_CREDENTIALS та доступність файлу credentials.json.", err)
	}

	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err)
	}
	log.Println("Клієнт Google Sheets успішно створено.")


	// Отримуємо оновлення через вебхук
	updates := bot.ListenForWebhook(webhookPath) // webhookPath тут використовується як шлях для HTTP-обробника

	// Запускаємо HTTP сервер для вебхука в окремій горутині
	go func() {
		log.Printf("Запуск HTTP сервера для вебхука на '%s', шлях: %s", cfg.WebhookListenAddr, webhookPath)
		// http.HandleFunc(webhookPath, func(w http.ResponseWriter, r *http.Request) {
		// 	// Цей обробник може бути потрібен, якщо ListenForWebhook не реєструє свій власний.
		// 	// Зазвичай ListenForWebhook сам обробляє запити на цей шлях.
		// 	// Якщо виникають проблеми, можливо, потрібно буде повернутися до явного http.HandleFunc.
		// 	log.Printf("Отримано запит на вебхук: %s %s", r.Method, r.URL.Path)
		// })
		err_http := http.ListenAndServe(cfg.WebhookListenAddr, nil) // Бот слухає HTTP на вказаній адресі
		if err_http != nil {
			// Ця помилка часто виникає, якщо порт вже зайнятий або немає прав.
			log.Fatalf("КРИТИЧНА ПОМИЛКА ЗАПУСКУ HTTP СЕРВЕРА для вебхука: %v", err_http)
		}
	}()

	log.Printf("Бот @%s готовий до роботи (слухає на %s, очікує запити від Nginx на %s)...", botUsername, cfg.WebhookListenAddr, webhookPath)
	telegram.HandleUpdates(updates, bot, sheetsService, cfg) // Передаємо cfg
}
