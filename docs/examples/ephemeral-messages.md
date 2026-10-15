# Ephemeral Messages

Telegram Bot API 10.2+ introduces ephemeral messages: messages that are visible only to a single recipient (`ReceiverUserID`) and the bot. Ephemeral messages can only be sent in group and supergroup chats, and their delivery is not guaranteed, especially if the recipient is offline.

## Sending an Ephemeral Message

Send configs that support ephemeral messages embed `EphemeralMessageParameters`. Set `ReceiverUserID` to make the message visible exclusively to that user; leaving it zero sends an ordinary message.

When the ephemeral message answers a callback query, also set `CallbackQueryID`. Setting `ReplaceCallbackQueryMessage` shows the ephemeral message in place of the message with the button, but it must be `false` when the callback query comes from an ephemeral message (such messages must be edited with the `EditEphemeralMessage*` configs instead, see below).

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
		cq := update.CallbackQuery
		if cq == nil {
			continue
		}

		// Acknowledge the callback query.
		callback := tgbotapi.NewCallback(cq.ID, "")
		if _, err := bot.Request(callback); err != nil {
			log.Println("callback error:", err)
		}

		// Message is nil for callback queries from inline messages, and
		// ephemeral messages can only be sent in group and supergroup chats.
		if cq.Message == nil || !(cq.Message.Chat.IsGroup() || cq.Message.Chat.IsSuperGroup()) {
			continue
		}

		// Send an ephemeral message visible only to the user who clicked the button.
		msg := tgbotapi.NewMessage(cq.Message.Chat.ID, "Only you can see this secret response!")
		msg.ReceiverUserID = cq.From.ID
		msg.CallbackQueryID = cq.ID
		// Replace the original message only if it is a regular message:
		// for callback queries from ephemeral messages this must be false.
		msg.ReplaceCallbackQueryMessage = cq.Message.EphemeralMessageID == 0

		sent, err := bot.Send(msg)
		if err != nil {
			log.Println("send error:", err)
			continue
		}

		// The returned Message carries the ephemeral identifier needed to
		// edit or delete the message later.
		log.Printf("Sent ephemeral message %d to user %d", sent.EphemeralMessageID, cq.From.ID)
	}
}
```

## Editing and Deleting Ephemeral Messages

Ephemeral messages are addressed by the chat they were sent to, the user who received them, and their per-chat ephemeral identifier: the `EphemeralMessageID` field of the `Message` returned by `bot.Send` (not `MessageID`). The identifier may be reused for another ephemeral message after the message is deleted or expires.

Use `bot.Request` to edit or delete ephemeral messages:

```go
msg := tgbotapi.NewMessage(chatID, "Only you can see this")
msg.ReceiverUserID = userID

sent, err := bot.Send(msg)
if err != nil {
	log.Panic(err)
}

// Edit the ephemeral message's text.
edit := tgbotapi.NewEditEphemeralMessageText(chatID, userID, sent.EphemeralMessageID, "Updated text")
if _, err := bot.Request(edit); err != nil {
	log.Panic(err)
}

// Delete the ephemeral message.
del := tgbotapi.NewDeleteEphemeralMessage(chatID, userID, sent.EphemeralMessageID)
if _, err := bot.Request(del); err != nil {
	log.Panic(err)
}
```

The same applies to buttons on an ephemeral message: the callback query's `Message.EphemeralMessageID` identifies the message to pass to the `EditEphemeralMessage*` configs.

## Replying to Ephemeral Messages

`ReplyParameters.EphemeralMessageID` is the identifier of an *incoming* ephemeral message in the current chat. A reply to an ephemeral message must itself be ephemeral (set `ReceiverUserID`), and may only be sent within 15 seconds of the original message:

```go
for update := range updates {
	incoming := update.Message
	if incoming == nil || incoming.EphemeralMessageID == 0 || incoming.From == nil {
		continue
	}

	reply := tgbotapi.NewMessage(incoming.Chat.ID, "Replying to your ephemeral message")
	// The reply must itself be ephemeral.
	reply.ReceiverUserID = incoming.From.ID
	reply.ReplyParameters = &tgbotapi.ReplyParameters{
		EphemeralMessageID: incoming.EphemeralMessageID,
	}

	if _, err := bot.Send(reply); err != nil {
		log.Println("reply error:", err)
	}
}
```
