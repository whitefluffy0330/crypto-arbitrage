package telegram

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/config"
	"github.com/whitefluffy0330/crypto-arbitrage/internal/sheets"
	gsheets "google.golang.org/api/sheets/v4"
)

// HandleInvestmentInput обробляє введення користувачем даних про інвестицію,
// парсить їх та намагається зберегти в Google Sheets.
func HandleInvestmentInput(bot *tgbotapi.BotAPI, message *tgbotapi.Message, srv *gsheets.Service, cfg config.Config) {
	chatID := message.Chat.ID
	inputText := message.Text

	log.Printf("Отримано текст для інвестиції від чату %d: %s", chatID, inputText)

	// Регулярний вираз для парсингу формату:
	// ТИП, НАЗВА, СУМА ВАЛЮТА, ДАТА (РРРР-ММ-ДД)
	// Приклад: Крипто-холд, BTC, 10000 USD, 2025-01-15
	// Групи:   (     1    ) (  2  ) (   3  ) ( 4 ) (    5     )
	re := regexp.MustCompile(`^([^,]+?)\s*,\s*([^,]+?)\s*,\s*(\d+(?:\.\d{1,2})?)\s*([а-яА-Яa-zA-Z]{3})\s*,\s*(\d{4}-\d{2}-\d{2})$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(inputText))

	var invData sheets.InvestmentData // Використовуємо структуру з пакета sheets
	var parsedSuccessfully bool
	var parseError error // Для збереження помилки парсингу

	if len(matches) == 6 { // Очікуємо 6 елементів: весь рядок + 5 груп
		invType := strings.TrimSpace(matches[1])
		invName := strings.TrimSpace(matches[2])
		amountStr := matches[3]
		currencyStr := strings.ToUpper(strings.TrimSpace(matches[4]))
		dateStr := matches[5]

		amount, errAmount := strconv.ParseFloat(amountStr, 64)
		// Парсимо дату, припускаючи, що користувач вводить її у форматі РРРР-ММ-ДД
		// Використовуємо KyivLocation для інтерпретації дати, але зберігатимемо в UTC
		dateInvested, errDate := time.ParseInLocation("2006-01-02", dateStr, sheets.KyivLocation)

		if errAmount == nil && errDate == nil {
			invData = sheets.InvestmentData{
				Type:           invType,
				Name:           invName,
				AmountInvested: amount,
				Currency:       currencyStr,
				DateInvested:   dateInvested.UTC(), // Зберігаємо в UTC
			}
			parsedSuccessfully = true
		} else {
			if errAmount != nil {
				parseError = fmt.Errorf("неправильна сума '%s'", amountStr)
			} else {
				parseError = fmt.Errorf("неправильний формат дати '%s' (очікується РРРР-ММ-ДД)", dateStr)
			}
		}
	} else {
		parseError = fmt.Errorf("неправильний загальний формат повідомлення")
	}

	var responseText string
	if parsedSuccessfully {
		// Викликаємо функцію для запису в таблицю
		err := sheets.AddInvestmentToSheet(srv, cfg.SpreadsheetID, cfg.SheetNameInvestments, chatID, invData)
		if err != nil {
			responseText = fmt.Sprintf("⚠️ Відбулася помилка під час збереження вашої інвестиції у Google Таблицю ('%s'):\n`%v`\n\nБудь ласка, перевірте налаштування таблиці та права доступу.", cfg.SheetNameInvestments, err)
			log.Printf("Помилка AddInvestmentToSheet для ChatID %d: %v", chatID, err)
		} else {
			responseText = fmt.Sprintf(
				"✅ Інвестицію успішно додано:\n\n"+
					"Тип: `%s`\n"+
					"Назва/Актив: `%s`\n"+
					"Сума: `%.2f %s`\n"+
					"Дата: `%s`",
				invData.Type, invData.Name, invData.AmountInvested, invData.Currency,
				invData.DateInvested.In(sheets.KyivLocation).Format("02.01.2006"), // Показуємо користувачеві в локальному часі
			)
			log.Printf("Інвестицію для ChatID %d успішно розпарсено та збережено в Google Sheets: %+v", chatID, invData)
		}
	} else {
		responseText = fmt.Sprintf("⚠️ Не вдалося розпізнати формат інвестиції.\nПомилка: %s.\n\n"+
			"Будь ласка, спробуйте ще раз у форматі:\n"+
			"`ТИП, НАЗВА, СУМА ВАЛЮТА, ДАТА (РРРР-ММ-ДД)`\n\n"+
			"Приклад: `Крипто-холд, BTC, 10000 USD, 2025-01-15`", parseError)
		log.Printf("Помилка парсингу інвестиції для ChatID %d: вхідний текст '%s', помилка: %s", chatID, inputText, parseError)
	}

	msg := tgbotapi.NewMessage(chatID, responseText)
	msg.ParseMode = tgbotapi.ModeMarkdown

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Помилка надсилання відповіді HandleInvestmentInput для чату %d: %v", chatID, err)
	}
}
