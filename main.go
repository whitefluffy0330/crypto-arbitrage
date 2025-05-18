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
	if bot == nil { // Додаткова перевірка, хоча InitBot має повернути помилку
		log.Fatal("Критична помилка: Не вдалося створити об'єкт бота (bot is nil).")
	}

	var botUsername string = "[ім'я невідоме]"
	// Після виправлень в InitBot, ми очікуємо, що bot.Self не буде nil, якщо InitBot не повернув помилку.
	if bot.Self != nil && bot.Self.ID != 0 {
		botUsername = bot.Self.UserName
	} else if bot.Self != nil && bot.Self.ID == 0 {
		log.Printf("ПОПЕРЕДЖЕННЯ (main.go): bot.Self.ID == 0, хоча bot.Self не nil. Ім'я користувача буде '[ім'я невідоме]'. UserName з API: '%s'", bot.Self.UserName)
	} else { // bot.Self == nil
		log.Printf("КРИТИЧНА ПОМИЛКА (main.go): bot.Self є nil після telegram.InitBot, хоча InitBot не повернув помилку. Це не мало статися. Перевірте логіку InitBot.")
		// Якщо InitBot не впорався з поверненням помилки, краще тут завершити роботу.
		// return
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
