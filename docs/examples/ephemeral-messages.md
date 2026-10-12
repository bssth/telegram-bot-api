# Ephemeral Messages

Telegram Bot API 10.2+ introduces ephemeral messages: messages sent to a chat that are visible only to a specific user (`ReceiverUserID`). They can also replace a callback query message when a user taps an inline button.

## Sending an Ephemeral Message

Set `ReceiverUserID` on your message configuration to make it visible exclusively to that user:

```go
package main

import (
	"log"
	"os"

	tgbotapi "github.com/bssth/telegram-bot-api/v6"
)

func main() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil {
		log.Panic(err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			// Acknowledge the callback query.
			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			if _, err := bot.Request(callback); err != nil {
				log.Println("callback error:", err)
			}

			// Send an ephemeral message visible only to the user who clicked the button.
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Only you can see this secret response!")
			msg.ReceiverUserID = update.CallbackQuery.From.ID
			msg.CallbackQueryID = update.CallbackQuery.ID
			msg.ReplaceCallbackQueryMessage = true

			if _, err := bot.Send(msg); err != nil {
				log.Println("send error:", err)
			}
		}
	}
}
```

## Editing and Deleting Ephemeral Messages

Because ephemeral messages are scoped to a specific recipient user, editing and deleting them requires both `ReceiverUserID` and `EphemeralMessageID`:

```go
// Edit an ephemeral message's text
edit := tgbotapi.NewEditEphemeralMessageText(chatID, receiverUserID, ephemeralMessageID, "Updated text")
if _, err := bot.Send(edit); err != nil {
	log.Panic(err)
}

// Delete an ephemeral message
del := tgbotapi.NewDeleteEphemeralMessage(chatID, receiverUserID, ephemeralMessageID)
if _, err := bot.Request(del); err != nil {
	log.Panic(err)
}
```

## Replying to Ephemeral Messages

To quote or reply to an ephemeral message, pass its ID in `ReplyParameters`:

```go
reply := tgbotapi.NewMessage(chatID, "Replying to your ephemeral message")
reply.ReplyParameters = &tgbotapi.ReplyParameters{
	EphemeralMessageID: ephemeralMessageID,
}

if _, err := bot.Send(reply); err != nil {
	log.Panic(err)
}
```
