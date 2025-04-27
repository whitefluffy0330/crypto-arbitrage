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
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

func main() {
	_ = godotenv.Load()

	token := os.Getenv("TELEGRAM_TOKEN")
	spreadsheetID := os.Getenv("SPREADSHEET_ID")
	sheetName := os.Getenv("SHEET_NAME")
	if token == "" || spreadsheetID == "" {
		log.Fatal("TELEGRAM_TOKEN або SPREADSHEET_ID не встановлені")
	}
	if sheetName == "" {
		sheetName = "Sheet1"
	}

	ctx := context.Background()
	credData, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Не вдалося прочитати credentials.json: %v", err)
	}
	config, err := google.JWTConfigFromJSON(credData, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Помилка аутентифікації Google Sheets: %v", err)
	}
	googleClient := config.Client(ctx)
	sheetsService, err := sheets.NewService(ctx, option.WithHTTPClient(googleClient))
	if err != nil {
		log.Fatalf("Не вдалося створити клієнта Google Sheets: %v", err)
	}

	keyboard := map[string]interface{}{
		"keyboard": [][]map[string]string{
			{{"text": "Почати роботу"}, {"text": "Закінчити роботу"}},
			{{"text": "Вихідний день"}},
		},
		"resize_keyboard":  true,
		"one_time_keyboard": false,
	}

	telegramAPI := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	app := fiber.New()

	app.Post("/webhook", func(c *fiber.Ctx) error {
		var update Update
		if err := json.Unmarshal(c.Body(), &update); err != nil {
			log.Printf("Помилка розбору оновлення: %v", err)
			return c.SendStatus(fiber.StatusBadRequest)
		}

		if update.Message != nil {
			msg := update.Message
			chatID := msg.Chat.ID
			text := msg.Text
			user := msg.From

			name := user.FirstName
			if user.LastName != "" {
				name += " " + user.LastName
			}
			if user.Username != "" {
				name = "@" + user.Username
			}

			log.Printf("Отримано повідомлення від %s: %s", name, text)

			responseText := ""
			switch text {
			case "/start":
				responseText = "Вітаю! Оберіть дію на клавіатурі:"
			case "Почати роботу", "Закінчити роботу", "Вихідний день":
				timestamp := time.Now().Format("2006-01-02 15:04:05")
				row := []interface{}{timestamp, name, text}
				vr := &sheets.ValueRange{
					Values: [][]interface{}{row},
				}
				_, err := sheetsService.Spreadsheets.Values.Append(spreadsheetID, sheetName+"!A:C", vr).ValueInputOption("RAW").Do()
				if err != nil {
					log.Printf("Помилка запису в Google Sheets: %v", err)
					responseText = "Помилка запису даних."
				} else {
					responseText = "✅ Дані збережено: " + text
				}
			default:
				responseText = "Невідома команда. Використовуйте кнопки!"
			}

			payload := map[string]interface{}{
				"chat_id": chatID,
				"text":    responseText,
			}
			if text == "/start" {
				payload["reply_markup"] = keyboard
			}
			payloadBytes, _ := json.Marshal(payload)
			http.Post(telegramAPI, "application/json", bytes.NewBuffer(payloadBytes))
		}

		return c.SendStatus(http.StatusOK)
	})

	log.Fatal(app.ListenTLS(":443", "/etc/letsencrypt/live/vadymnewchapter.pp.ua/fullchain.pem", "/etc/letsencrypt/live/vadymnewchapter.pp.ua/privkey.pem"))
}
