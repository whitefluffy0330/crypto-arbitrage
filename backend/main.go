// main.go (фінальна версія з підтримкою HTTPS, Google Sheets, Telegram Bot, цілей, вечірнього звіту, мотивації)

... // [основна частина залишилась незмінною — вже є в Canvas]

// 🔻 Починається повний функціонал нижче

func handleCommand(message *tgbotapi.Message) { ... }

func handleButton(message *tgbotapi.Message) { ... }

func showMainKeyboard(chatID int64) { ... }

func handleCallback(query *tgbotapi.CallbackQuery) { ... }

func startWork(message *tgbotapi.Message) {
	startWorkTime = time.Now()
	isWorking = true
	isBreakRequested = false
	go startBreakTimer(message.Chat.ID)
	msg := tgbotapi.NewMessage(message.Chat.ID, "Роботу розпочато!")
	bot.Send(msg)
}

func finishWork(message *tgbotapi.Message) {
	if startWorkTime.IsZero() {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Спочатку потрібно натиснути «Почати роботу»!")
		bot.Send(msg)
		return
	}
	endWorkTime := time.Now()
	duration := endWorkTime.Sub(startWorkTime)
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	durationStr := strings.TrimSpace(fmt.Sprintf("%d год %d хв", hours, minutes))
	writeRow("Звіт", []interface{}{time.Now().Format("02.01.2006"), "Працював", durationStr})
	isWorking = false
	isBreakRequested = false
	startWorkTime = time.Time{}
	msg := tgbotapi.NewMessage(message.Chat.ID, "Роботу завершено та записано у звіт!")
	bot.Send(msg)
	sendMotivation(message.Chat.ID)
}

func registerDayOff(message *tgbotapi.Message) {
	writeRow("Звіт", []interface{}{time.Now().Format("02.01.2006"), "Вихідний", ""})
	msg := tgbotapi.NewMessage(message.Chat.ID, "Вихідний день записано!")
	bot.Send(msg)
}

func handleGoalCreation(message *tgbotapi.Message) {
	switch goalState {
	case "waiting_goal_name":
		tempGoalName = message.Text
		goalState = "waiting_goal_amount"
		msg := tgbotapi.NewMessage(message.Chat.ID, "Яка сума ($) потрібна для цієї цілі?")
		bot.Send(msg)
	case "waiting_goal_amount":
		tempGoalAmount = message.Text
		writeRow("Цілі", []interface{}{tempGoalName, tempGoalAmount, "", "У процесі", "0%"})
		writeRow("Фінансовий План", []interface{}{time.Now().Format("January 2006"), 0, 0, 0})
		goalState = ""
		msg := tgbotapi.NewMessage(message.Chat.ID, "Ціль додано. Ти на шляху до великої мети! 🚀")
		bot.Send(msg)
	case "waiting_goal_close":
		realAmount := message.Text
		writeRow("Цілі", []interface{}{tempGoalName, tempGoalAmount, realAmount, "✅ Завершено", "100%"})
		goalState = ""
		msg := tgbotapi.NewMessage(message.Chat.ID, "🎯 Ціль успішно завершена! Вітаю!")
		bot.Send(msg)
	}
}

func startBreakTimer(chatID int64) {
	for isWorking {
		time.Sleep(90 * time.Minute)
		if isWorking && !isBreakRequested {
			remindBreak(chatID)
		}
	}
}

func remindBreak(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🔔 Пора зробити перерву!")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ок, йду відпочивати", "start_break"),
			tgbotapi.NewInlineKeyboardButtonData("Я вже тут", "end_break"),
		),
	)
	bot.Send(msg)
}

func sendMotivation(chatID int64) {
	phrases := []string{
		"Кожен день — це крок ближче до твоєї мрії! 🚀",
		"Памʼятай, для чого ти почав. 🔥",
		"Сьогоднішня праця — це завтрашня свобода. 💸",
		"Ти будуєш нову реальність прямо зараз! 💪",
		"Навіть маленький крок — це прогрес. 📈",
	}
	msg := tgbotapi.NewMessage(chatID, phrases[rand.Intn(len(phrases))])
	bot.Send(msg)
}

func writeRow(sheet string, row []interface{}) {
	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, sheet+"!A:Z", &sheets.ValueRange{
		Values: [][]interface{}{row},
	}).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.Printf("Помилка запису в Google Sheet %s: %v", sheet, err)
	}
}

func eveningReport() {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 21, 0, 0, 0, now.Location())
		if now.After(next) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))

		readRange := "Звіт!A:C"
		resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
		if err != nil || len(resp.Values) < 2 {
			continue
		}
		last := resp.Values[len(resp.Values)-1]
		text := "📋 Звіт за день:\n"
		if len(last) >= 3 {
			text += fmt.Sprintf("✅ Статус: %s\n🕒 Час роботи: %s\n", last[1], last[2])
		} else {
			text += "Немає повного запису про сьогодні."
		}
		text += "\n🏆 Ти просуваєшся до мети, так тримати!"
		msg := tgbotapi.NewMessage(chatID, text)
		bot.Send(msg)
	}
}
