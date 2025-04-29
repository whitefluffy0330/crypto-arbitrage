
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/joho/godotenv"
)

var (
    botToken   string
    chatID     string
    webhookURL string
    coingeckoAPI = "https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&order=market_cap_desc&per_page=100&page=1&sparkline=false"
)

type Update struct {
    UpdateID int `json:"update_id"`
    Message  struct {
        MessageID int `json:"message_id"`
        From      struct {
            ID int `json:"id"`
        } `json:"from"`
        Chat struct {
            ID int64 `json:"id"`
        } `json:"chat"`
        Text string `json:"text"`
    } `json:"message"`
    CallbackQuery struct {
        ID   string `json:"id"`
        Data string `json:"data"`
        From struct {
            ID int `json:"id"`
        } `json:"from"`
        Message struct {
            MessageID int `json:"message_id"`
            Chat      struct {
                ID int64 `json:"id"`
            } `json:"chat"`
        } `json:"message"`
    } `json:"callback_query"`
}

type Coin struct {
    ID              string  `json:"id"`
    Symbol          string  `json:"symbol"`
    Name            string  `json:"name"`
    CurrentPrice    float64 `json:"current_price"`
    MarketCap       float64 `json:"market_cap"`
    TotalVolume     float64 `json:"total_volume"`
    PriceChange24h  float64 `json:"price_change_24h"`
    PriceChangePerc float64 `json:"price_change_percentage_24h"`
}

func main() {
    err := godotenv.Load()
    if err != nil {
        log.Fatalf("Помилка завантаження .env файлу: %v", err)
    }

    botToken = os.Getenv("TELEGRAM_TOKEN")
    chatID = os.Getenv("TELEGRAM_CHAT_ID")
    webhookURL = os.Getenv("WEBHOOK_URL")

    if botToken == "" || chatID == "" || webhookURL == "" {
        log.Fatal("Будь ласка, перевір .env файл: TELEGRAM_TOKEN, TELEGRAM_CHAT_ID або WEBHOOK_URL відсутні.")
    }

    setWebhook()

    http.HandleFunc("/webhook", handleWebhook)

    log.Println("Сервер запущено на порті 8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func setWebhook() {
    url := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", botToken)
    data := map[string]string{
        "url": webhookURL,
    }
    body, _ := json.Marshal(data)

    resp, err := http.Post(url, "application/json", bytes.NewReader(body))
    if err != nil {
        log.Fatalf("Помилка встановлення webhook: %v", err)
    }
    defer resp.Body.Close()

    log.Println("Webhook успішно встановлено")
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
    var update Update
    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        log.Println("Помилка читання тіла запиту:", err)
        return
    }
    defer r.Body.Close()

    err = json.Unmarshal(body, &update)
    if err != nil {
        log.Println("Помилка розбору JSON:", err)
        return
    }

    if update.Message.Text != "" {
        log.Printf("Отримано повідомлення: %s", update.Message.Text)
        processMessage(update.Message.Chat.ID, update.Message.Text)
    } else if update.CallbackQuery.Data != "" {
        log.Printf("Натиснута кнопка: %s", update.CallbackQuery.Data)
        sendMessage(update.CallbackQuery.Message.Chat.ID, "Ви натиснули кнопку: "+update.CallbackQuery.Data)
    }
}

func processMessage(chatID int64, text string) {
    switch text {
    case "/start":
        sendMessage(chatID, "🤖 Ласкаво просимо! Бот працює. Ви можете використовувати команди для моніторингу ринку.")
    case "/topcoins":
        coins := getTopCoins()
        message := "🔥 ТОП-10 монет:

"
        for i, coin := range coins[:10] {
            message += fmt.Sprintf("%d. %s (%s): $%.2f
", i+1, coin.Name, coin.Symbol, coin.CurrentPrice)
        }
        sendMessage(chatID, message)
    default:
        sendMessage(chatID, "Команда не розпізнана. Використайте /start або /topcoins.")
    }
}

func getTopCoins() []Coin {
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Get(coingeckoAPI)
    if err != nil {
        log.Println("Помилка запиту CoinGecko:", err)
        return []Coin{}
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Println("Помилка читання відповіді CoinGecko:", err)
        return []Coin{}
    }

    var coins []Coin
    if err := json.Unmarshal(body, &coins); err != nil {
        log.Println("Помилка парсингу JSON CoinGecko:", err)
        return []Coin{}
    }

    return coins
}

func sendMessage(chatID int64, text string) {
    url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
    body := map[string]interface{}{
        "chat_id":    chatID,
        "text":       text,
        "parse_mode": "HTML",
    }
    jsonBody, _ := json.Marshal(body)

    _, err := http.Post(url, "application/json", bytes.NewReader(jsonBody))
    if err != nil {
        log.Println("Помилка надсилання повідомлення:", err)
    }
}
