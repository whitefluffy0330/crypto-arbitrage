package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Структури для отримання даних від Telegram
type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      *User  `json:"from"`
	Chat      *Chat  `json:"chat"`
	Date      int    `json:"date"`
	Text      string `json:"text"`
}

type User struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Username string `json:"username"`
}

// Структури для відправки повідомлення з клавіатурою
type ReplyKeyboardMarkup struct {
	Keyboard        [][]KeyboardButton `json:"keyboard"`
	ResizeKeyboard  bool               `json:"resize_keyboard"`
	OneTimeKeyboard bool               `json:"one_time_keyboard"`
}

type KeyboardButton struct {
	Text string `json:"text"`
}

func main() {
	// Завантаження змінних середовища з .env (якщо файл існує)
	_ = godotenv.Load()

	// Зчитування необхідних змінних
	token := os.Getenv("TELEGRAM_TOKEN")
	spreadsheetID := os.Getenv("SPREADSHEET_ID")
	sheetName := os.Getenv("SHEET_NAME")
	if token == "" || spreadsheetID == "" {
		log.Fatal("Необхідно встановити TELEGRAM_TOKEN та SPREADSHEET_ID у файлі .env або змінних середовища")
	}
	if sheetName == "" {
		sheetName = "Sheet1"
	}

	// Підключення до Google Sheets
	ctx := context.Background()
	credData, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Не вдалося прочитати файл credentials.json: %v", err)
	}
	config, err := google.JWTConfigFromJSON(credData, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Помилка аутентифікації Google Sheets: %v", err)
	}
	googleClient := config.Client(ctx)
	sheetsService, err := sheets.NewService(ctx, option.WithHTTPClient(googleClient))
	if err != nil {
		log.Fatalf("Не вдалося підключитися до сервісу Google Sheets: %v", err)
	}

	// Налаштування варіантів клавіатури
	keyboard := ReplyKeyboardMarkup{
		Keyboard: [][]KeyboardButton{
			{{Text: "Почати роботу"}, {Text: "Закінчити роботу"}},
			{{Text: "Вихідний день"}},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: false,
	}

	// URL для виклику API Telegram sendMessage
	telegramAPI := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	// Створення Fiber додатку
	app := fiber.New()

	// Маршрут для обробки запитів від Telegram (webhook)
	// (Не забудьте встановити webhook для вашого бота на URL цього маршруту)
	app.Post("/webhook", func(c *fiber.Ctx) error {
		var update Update
		if err := c.BodyParser(&update); err != nil {
			log.Printf("Помилка розбору запиту: %v", err)
			return c.SendStatus(fiber.StatusBadRequest)
		}

		// Перевірка наявності повідомлення
		if update.Message != nil {
			msg := update.Message
			chatID := msg.Chat.ID
			user := msg.From
			text := msg.Text

			// Ім'я користувача для логів та запису (username або ім'я та прізвище)
			nameLog := user.FirstName
			if user.LastName != "" {
				nameLog += " " + user.LastName
			}
			if user.Username != "" {
				// Використати username, якщо є
				nameLog = "@" + user.Username
			}

			log.Printf("Отримано повідомлення від %s: %s", nameLog, text)

			// Підготувати відповідь
			responseText := ""
			includeKeyboard := false

			switch text {
			case "/start":
				// Привітальне повідомлення з клавіатурою
				responseText = "Вітаю! Оберіть опцію на клавіатурі нижче:"
				includeKeyboard = true

			case "Почати роботу", "Закінчити роботу", "Вихідний день":
				// Реєстрація події у Google Sheets
				timestamp := time.Now().Format("2006-01-02 15:04:05")
				// Визначити назву дії (залишити як текст кнопки)
				action := text
				// Ім'я для запису (username або ім'я і прізвище)
				name := user.FirstName
				if user.LastName != "" {
					name += " " + user.LastName
				}
				if user.Username != "" {
					name = "@" + user.Username
				}
				row := []interface{}{timestamp, name, action}
				vr := &sheets.ValueRange{
					Values: [][]interface{}{row},
				}
				_, err := sheetsService.Spreadsheets.Values.Append(spreadsheetID, sheetName+"!A:C", vr).ValueInputOption("RAW").Do()
				if err != nil {
					log.Printf("Помилка запису в Google Sheets: %v", err)
					responseText = "Помилка збереження даних."
				} else {
					log.Printf("Записано в Google Sheets: %s - %s", name, action)
					// Повідомлення-підтвердження для користувача
					switch text {
					case "Почати роботу":
						responseText = "✅ Робочий день розпочато"
					case "Закінчити роботу":
						responseText = "✅ Робочий день завершено"
					case "Вихідний день":
						responseText = "✅ Вихідний день зафіксовано"
					}
				}

			default:
				// Невідома команда
				responseText = "Невідома команда. Скористайтесь кнопками на клавіатурі."
			}

			// Відправка відповіді назад у Telegram
			if responseText != "" {
				payload := map[string]interface{}{
					"chat_id": chatID,
					"text":    responseText,
				}
				if includeKeyboard {
					payload["reply_markup"] = keyboard
				}
				payloadBytes, _ := json.Marshal(payload)
				resp, err := http.Post(telegramAPI, "application/json", bytes.NewBuffer(payloadBytes))
				if err != nil {
					log.Printf("Помилка при відправці повідомлення в Telegram: %v", err)
				} else {
					defer resp.Body.Close()
					if resp.StatusCode != 200 {
						log.Printf("Неочікуваний статус відповіді Telegram API: %d", resp.StatusCode)
					}
				}
			}
		}

		// HTTP 200 відповідь для підтвердження отримання вебхука
		return c.SendStatus(fiber.StatusOK)
	})

	// Запуск сервера HTTPS
	// Зверніть увагу: для роботи потрібні файли сертифіката "cert.pem" та ключа "key.pem"
	if err := app.ListenTLS(":443", "cert.pem", "key.pem"); err != nil {
		log.Fatalf("Помилка запуску сервера: %v", err)
	}
}
