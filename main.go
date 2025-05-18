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
	// "github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // Цей імпорт тут не потрібен, sheets.SpreadsheetsScope використовується в telegram.SetWebhook/InitBot, якщо там є логіка з sheets
	// або в telegram.HandleUpdates, якщо sheetsService передається

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
	// Після змін в InitBot, ми покладаємося на те, що він або повернув помилку,
	// або bot.Self.ID має якесь значення (можливо 0, якщо GetMe не вдалося).
	if bot.Self.ID != 0 {
		botUsername = bot.Self.UserName
	} else {
		log.Printf("ПОПЕРЕДЖЕННЯ (main.go): bot.Self.ID == 0 після InitBot. Username: '%s'.", bot.Self.UserName)
	}
	log.Printf("Бот @%s ініціалізовано.", botUsername)

	webhookPath := cfg.WebhookPath
	if !strings.HasPrefix(webhookPath, "/") && webhookPath != "" {
		webhookPath = "/" + webhookPath
	}

	// Виклик SetWebhook з параметрами URL, шлях, шлях до сертифіката
	err = telegram.SetWebhook(bot, cfg.WebhookBaseURL, webhookPath, cfg.WebhookCertPath)
	if err != nil {
		log.Printf("ПОПЕРЕДЖЕННЯ/ПОМИЛКА встановлення вебхука: %v.", err)
	}

	ctx := appContext()
	// sheets.SpreadsheetsScope має бути визначено в пакеті sheets
	// або використовуйте gsheets.SpreadsheetsScope безпосередньо, якщо це те саме
	credentials, err := google.FindDefaultCredentials(ctx, gsheets.SpreadsheetsScope) // Використовуємо gsheets.SpreadsheetsScope
	if err != nil {
		log.Fatalf("Помилка авторизації Google Sheets (FindDefaultCredentials): %v. Перевірте GOOGLE_APPLICATION_CREDENTIALS.", err)
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
				log.Fatalf("КРИТИЧНА ПОМИЛКА ЗАПУСКУ HTTP СЕРВЕРА: %v", err_http)
			}
		}()
		log.Printf("Бот @%s готовий до роботи (слухає на %s, очікує запити від Nginx на %s)...", botUsername, cfg.WebhookListenAddr, webhookPath)
	} else {
		log.Println("ПОПЕРЕДЖЕННЯ: WebhookPath не вказано. Вебхук не слухається.")
	}

	if updatesChannel != nil {
		telegram.HandleUpdates(updatesChannel, bot, sheetsService, cfg)
	} else {
		log.Println("Канал оновлень не ініціалізовано. Зупинка.")
	}
}
