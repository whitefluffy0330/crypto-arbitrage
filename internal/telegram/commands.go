package telegram

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

func StartWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message)                                    {}
func StopWork(bot *tgbotapi.BotAPI, msg *tgbotapi.Message)                                     {}
func DayOff(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv any, spreadsheetID string)        {}
func ShowMainKeyboard(bot *tgbotapi.BotAPI, cfg any, chatID int64)                             {}
func HandleButtons(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, srv any, spreadsheetID string) {}
func HandleCallback(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery)                          {}
