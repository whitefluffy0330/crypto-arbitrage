azureuser@newchapter-rg-copy:~/crypto-arbitrage$ cd ~/crypto-arbitrage 
git fetch origin && git reset --hard origin/main
go mod tidy
go build -o mycryptobot main.go
sudo systemctl restart cryptobot.service
sudo journalctl -u cryptobot.service -f
Username for 'https://github.com': whitefluffy0330
Password for 'https://whitefluffy0330@github.com': 
remote: Enumerating objects: 9, done.
remote: Counting objects: 100% (9/9), done.
remote: Compressing objects: 100% (5/5), done.
remote: Total 5 (delta 4), reused 0 (delta 0), pack-reused 0 (from 0)
Unpacking objects: 100% (5/5), 1.27 KiB | 326.00 KiB/s, done.
From https://github.com/whitefluffy0330/crypto-arbitrage
   e8cc2e6..b804895  main       -> origin/main
HEAD is now at b804895 Update handler.go
# github.com/whitefluffy0330/crypto-arbitrage/internal/telegram
internal/telegram/handler.go:479:45: too many arguments in call to HandleSpreadsCommand
	have (*tgbotapi.BotAPI, int64, config.Config, number)
	want (*tgbotapi.BotAPI, int64, config.Config)
May 16 13:32:23 newchapter-rg-copy mycryptobot[92485]: 2025/05/16 13:32:23 Для ChatID -1002661053556 поріг фандингу не встановлено, використовується стандартний 0.0005%
May 16 13:47:28 newchapter-rg-copy systemd[1]: Stopping Crypto Arbitrage Bot...
May 16 13:47:28 newchapter-rg-copy systemd[1]: cryptobot.service: Deactivated successfully.
May 16 13:47:28 newchapter-rg-copy systemd[1]: Stopped Crypto Arbitrage Bot.
May 16 13:47:28 newchapter-rg-copy systemd[1]: Started Crypto Arbitrage Bot.
May 16 13:47:28 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:47:28 Часову зону Europe/Kyiv завантажено (sheets).
May 16 13:47:28 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:47:28 Часову зону Europe/Kyiv завантажено (telegram).
May 16 13:47:28 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:47:28 ПОПЕРЕДЖЕННЯ: Змінна TELEGRAM_CHAT_ID не встановлена. ChatID буде 0.
May 16 13:47:28 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:47:28 Конфігурацію завантажено: SpreadsheetID=1NWEUTF4d6X7xAIi2Bs_mz2rcLWkCdftQzUhFq0NX8bU, ReportSheet='Звіт!A2:E2', GoalsSheet='МоїЦілі', WorkLogSheet='РобочийГрафік', InvestmentsSheet='Інвестиції', Webhook=https://vadymnewchapter.pp.ua/0xc23701CFde4a26e5Be787dE2673160d91b423732, BotListenAddr=localhost:8080, AdminChatID=0
May 16 13:47:28 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:47:28 Спроба ініціалізації бота через tgbotapi.NewBotAPI...
May 16 13:47:28 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:47:28 Бот @W8arbitrageBOT ініціалізовано.
May 16 13:47:28 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:47:28 Бот @W8arbitrageBOT готовий до роботи (слухає на localhost:8080, очікує запити від Nginx на /0xc23701CFde4a26e5Be787dE2673160d91b423732)...
May 16 13:47:28 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:47:28 Розпочато обробку оновлень...
May 16 13:47:28 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:47:28 Запуск HTTP сервера для вебхука на 'localhost:8080', шлях: /0xc23701CFde4a26e5Be787dE2673160d91b423732
May 16 13:48:50 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:48:50 [vd0330] (-1002661053556): 💹 Funding Rates
May 16 13:48:50 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:48:50 Обробка команди /funding для ChatID: -1002661053556
May 16 13:48:50 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:48:50 Запит до Binance API: https://fapi.binance.com/fapi/v1/premiumIndex
May 16 13:48:51 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:48:51 Отримано дані фінансування для 499 символів з Binance.
May 16 13:49:01 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:49:01 [vd0330] (-1002661053556): 💹 Funding Rates
May 16 13:49:01 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:49:01 Обробка команди /funding для ChatID: -1002661053556
May 16 13:49:01 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:49:01 Запит до Binance API: https://fapi.binance.com/fapi/v1/premiumIndex
May 16 13:49:01 newchapter-rg-copy mycryptobot[94133]: 2025/05/16 13:49:01 Отримано дані фінансування для 499 символів з Binance.

