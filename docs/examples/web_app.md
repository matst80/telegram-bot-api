# Web Apps Example

This example demonstrates how to use Telegram Web Apps with `go-telegram-bot-api`.

## 1. Opening a Web App from a Keyboard

You can use `WebAppInfo` inside a `KeyboardButton` to open a Web App from the reply keyboard.

```go
package main

import (
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v6" // assuming v6 or latest
)

func main() {
	bot, _ := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))

	// Create a reply keyboard button with WebAppInfo
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.KeyboardButton{
				Text:   "Open Web App",
				WebApp: &tgbotapi.WebAppInfo{URL: "https://example.com/webapp"},
			},
		),
	)

	// Send a message with the keyboard
	msg := tgbotapi.NewMessage(123456, "Click the button below to open the Web App.")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}
```

## 2. Receiving Data from Web App

When a user interacts with the Web App and sends data back using `Telegram.WebApp.sendData()`, the bot receives a service message with `WebAppData`.

```go
for update := range updates {
	if update.Message != nil && update.Message.WebAppData != nil {
		data := update.Message.WebAppData.Data
		log.Printf("Received data from Web App: %s", data)
		
		// Respond to the user
		reply := tgbotapi.NewMessage(update.Message.Chat.ID, "Got your data: "+data)
		bot.Send(reply)
	}
}
```

## 3. Answering a Web App Query

If the Web App was opened via an inline button and uses `answerWebAppQuery` on the client side, the bot can answer it using `AnswerWebAppQuery`.

```go
// Using AnswerWebAppQueryConfig
config := tgbotapi.AnswerWebAppQueryConfig{
	WebAppQueryID: "some_query_id", // From the Web App client-side logic
	Result: tgbotapi.NewInlineQueryResultArticle(
		"result_id",
		"Success!",
		"The Web App query was answered successfully.",
	),
}

if _, err := bot.AnswerWebAppQuery(config); err != nil {
	log.Printf("Error: %v", err)
}
```
