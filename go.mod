module github.com/whitefluffy0330/crypto-arbitrage

go 1.22

require (
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/gofiber/fiber/v2 v2.52.6
	github.com/joho/godotenv v1.5.1
	golang.org/x/oauth2 v0.17.0
	google.golang.org/api v0.180.0
)

replace github.com/whitefluffy0330/crypto-arbitrage/internal/config => ./internal/config
replace github.com/whitefluffy0330/crypto-arbitrage/internal/sheets => ./internal/sheets
replace github.com/whitefluffy0330/crypto-arbitrage/internal/telegram => ./internal/telegram
replace github.com/whitefluffy0330/crypto-arbitrage/internal/telegram/goal => ./internal/telegram/goal
