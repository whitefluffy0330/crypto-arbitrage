package main

import (
	"context"
	"log"
	"net/http"
	// "time" // Наразі не використовується тут безпосередньо

	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets" // Для sheets.SpreadsheetsScope
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/motivation" // Для InitMotivationSeed

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	gsheets "google.golang.org/api/sheets/v4" // Перейменовано імпорт, щоб уникнути конфлікту з вашим пакетом sheets
)

// appContext повертає фоновий контекст.
func appContext() context.Context {
	return context.Background()
}

func main() {
	// 1. Ініціалізація насіння для генератора випадкових чисел (для мотиваційних фраз)
	motivation.InitMotivationSeed()

	// 2. Завантаження конфігурації
	cfg := config.LoadEnv()

	// 3. Перевірка обов'язкових конфігураційних параметрів
	// cfg.ChatID може бути не потрібен для загального запуску, а для конкретних повідомлень (наприклад, звітів конкретному адміну)
	if cfg.BotToken == "" || cfg.SpreadsheetID == "" {
		log.Fatal("Критична помилка: Не задані обов'язкові змінні середовища TELEGRAM_TOKEN та SPREADSHEET_ID")
	}
	// Якщо cfg.ChatID використовується для чогось глобального, залиште перевірку:
	// if cfg.ChatID == 0 {
	// 	log.Fatal("Критична помилка: Не задана змінна середовища TELEGRAM_CHAT_ID")
	// }


	// 4. Ініціалізація Telegram бота (використовуємо спрощену функцію з пакета telegram)
	bot, err := telegram.InitBot(cfg.BotToken)
	if err != nil {
		log.Fatalf("Помилка ініціалізації бота: %v", err)
	}

	// 5. Налаштування та встановлення Webhook
	// TODO: Розгляньте можливість винесення цих URL та шляхів до сертифікатів у конфігурацію (cfg)
	webhookBaseURL := "https://vadymnewchapter.pp.ua" // ВАШ ПУБЛІЧНИЙ ДОМЕН З HTTPS
	// Створюємо унікальний шлях для вебхука, щоб його було важче вгадати. Можна додати частину токена.
	webhookPath := "/webhook_" + bot.Token 
	
	// Шлях до публічного сертифіката для Telegram (якщо потрібен для NewWebhookWithCert).
	// Для Let's Encrypt, якщо сервер правильно налаштований, Telegram зазвичай не потребує цього,
	// тому можна передавати порожній рядок.
	// certFilePath := "/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem"
	certFilePath := "" // Залиште порожнім для Let's Encrypt з надійним CA

	err = telegram.SetWebhook(bot, webhookBaseURL, webhookPath, certFilePath)
	if err != nil {
		log.Fatalf("Помилка встановлення вебхука: %v", err)
	}

	// 6. Ініціалізація клієнта Google Sheets
	ctx := appContext()
	// Переконайтеся, що змінна середовища GOOGLE_APPLICATION_CREDENTIALS правильно встановлена на вашому сервері
	// і вказує на файл з ключами, який знаходиться ПОЗА репозиторієм.
	credentials, err := google.FindDefaultCredentials(ctx, sheets.SpreadsheetsScope) // Використовуємо sheets.SpreadsheetsScope з вашого пакета
	if err != nil {
		log.Fatalf("Помилка авторизації Google Sheets (перевірте GOOGLE_APPLICATION_CREDENTIALS): %v", err)
	}

	sheetsService, err := gsheets.NewService(ctx, option.WithCredentials(credentials)) // Використовуємо gsheets.NewService
	if err != nil {
		log.Fatalf("Не вдалося створити клієнт Google Sheets: %v", err)
	}

	// 7. Отримання оновлень від Telegram (канал updates)
	// bot.ListenForWebhook реєструє обробник на http.DefaultServeMux для шляху webhookPath
	updates := bot.ListenForWebhook(webhookPath) // Шлях має ТОЧНО співпадати з тим, що встановлено у SetWebhook

	// 8. Запуск HTTPS сервера для приймання запитів від Telegram
	go func() {
		log.Printf("Запуск HTTPS сервера для вебхука на порту 443, шлях: %s", webhookPath)
		// TODO: Розгляньте можливість винесення шляхів до сертифікатів у конфігурацію.
		// Переконайтеся, що ці шляхи правильні та файл сертифіката/ключа доступний для читання.
		// Для роботи на стандартному порту 443 потрібні права суперкористувача,
		// або використовуйте реверс-проксі (наприклад, Nginx), який слухає 443 і перенаправляє на інший порт вашого застосунку.
		err_https := http.ListenAndServeTLS(":443",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem",
			"/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem",
			nil) // nil означає використання http.DefaultServeMux
		if err_https != nil {
			log.Fatalf("Помилка запуску HTTPS сервера: %v", err_https)
		}
	}()

	log.Printf("Бот @%s готовий до роботи та очікує на оновлення через вебхук...", bot.Self.UserName)

	// 9. Запуск функції для вечірніх звітів (якщо вона є та потрібна)
	// telegram.StartEveningReport(bot, sheetsService, cfg.SpreadsheetID, cfg.ChatID) // Ми ще не бачили реалізацію цієї функції

	// 10. Передача оновлень на обробку в головний цикл (передаємо cfg.SpreadsheetID)
	telegram.HandleUpdates(updates, bot, sheetsService, cfg.SpreadsheetID)
}
