package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	// tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5" // Імпорт tgbotapi тут не потрібен, якщо bot передається
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"         // Для sheets.SpreadsheetsScope, якщо він там визначений
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation" // Для ініціалізації

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4" // Аліас для офіційного пакета Google Sheets
)

func appContext() context.Context { return context.Background() }

func main() {
	motivation.InitMotivationSeed()
	cfg := config.LoadEnv() // Завантажуємо конфігурацію

	// Ініціалізація бота
	bot, err := telegram.InitBot(cfg.BotToken)
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}
	// Згідно з виправленою InitBot, якщо err == nil, то bot != nil і bot.Self != nil.

	var botUsername string = "[ім'я невідоме]"
	// У виправленій InitBot вже є логування стану bot.Self.
	// Тут ми просто використовуємо дані, якщо вони доступні.
	if bot.Self.ID != 0 {
		botUsername = bot.Self.UserName
	} else {
		// Це логування може бути корисним, якщо InitBot пройшов, але ID все одно 0
		log.Printf("ПОПЕРЕДЖЕННЯ (main.go): bot.Self.ID все ще 0 після InitBot, хоча InitBot не повернув помилку. UserName з API: '%s'.", bot.Self.UserName)
	}
	log.Printf("Бот @%s ініціалізовано.", botUsername)

	// Налаштування вебхука
	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") && webhookPath != "" { // Додано перевірку на порожній webhookPath
		webhookPath = "/" + webhookPath
	}

	// WebhookCertPath має бути порожнім, якщо Nginx обробляє TLS (це налаштовується у /etc/crypto-bot/environment)
	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, webhookPath, cfg.WebhookCertPath)
	if err != nil {
		// Не робимо Fatal, оскільки бот може працювати в режимі polling або вебхук вже встановлено
		log.Printf("ПОПЕРЕДЖЕННЯ/ПОМИЛКА встановлення вебхука: %v. Бот продовжить роботу, але вебхук може бути неактивним.", err)
	}

	// Ініціалізація Google Sheets API
	ctx := appContext()
	// sheets.SpreadsheetsScope має бути "https://www.googleapis.com/auth/spreadsheets"
	// Якщо він визначений у вашому пакеті sheets, це коректно.
	// Альтернативно, можна використовувати gsheets.SpreadsheetsScope напряму.
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
	// bot.ListenForWebhook має повертати tgbotapi.UpdatesChannel
	// Перевіряємо, чи webhookPath не порожній, перш ніж слухати
	if webhookPath != "" {
		updates := bot.ListenForWebhook(webhookPath)

		// Запускаємо HTTP сервер для вебхука в окремій горутині
		go func() {
			log.Printf("Запуск HTTP сервера для вебхука на '%s', шлях: %s", cfg.WebhookListenAddr, webhookPath)
			err_http := http.ListenAndServe(cfg.WebhookListenAddr, nil)
			if err_http != nil {
				log.Fatalf("КРИТИЧНА ПОМИЛКА ЗАПУСКУ HTTP СЕРВЕРА для вебхука: %v", err_http)
			}
		}()

		log.Printf("Бот @%s готовий до роботи (слухає на %s, очікує запити від Nginx на %s)...", botUsername, cfg.WebhookListenAddr, webhookPath)
		telegram.HandleUpdates(updates, bot, sheetsService, cfg) // Передаємо cfg
	} else {
		log.Println("ПОПЕРЕДЖЕННЯ: WebhookPath не вказано в конфігурації. Бот не буде слухати вебхуки. Робота в режимі polling не реалізована.")
		// Якщо не вебхук, то програма просто завершиться, якщо немає іншої логіки.
	}
}
