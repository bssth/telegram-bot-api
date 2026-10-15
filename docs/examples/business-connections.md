# Business Connections

Telegram Business accounts can connect bots to manage customer chats and respond on behalf of the business.

## Receiving Connection Updates

When a user connects or disconnects a bot with their business account, Telegram delivers an update with `update.BusinessConnection`:

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
		if update.BusinessConnection != nil {
			bc := update.BusinessConnection
			log.Printf("Business connection %s from user %d (enabled: %t)", bc.ID, bc.User.ID, bc.IsEnabled)

			if bc.Rights != nil {
				log.Printf("Can reply: %t", bc.Rights.CanReply)
			}
		}
	}
}
```

## Handling Business Messages

Messages in the chats of a connected business account arrive in `update.BusinessMessage`. This includes the account owner's own outgoing messages and messages sent on the owner's behalf by a business bot (`SenderBusinessBot`), so an auto-reply should skip those and only answer messages from other users. The owner is the `User` of the `BusinessConnection`.

To send a message or reply on behalf of the business, assign the `BusinessConnectionID` from the incoming message:

```go
// owners maps each business connection ID to the ID of the business
// account owner.
owners := make(map[string]int64)

for update := range updates {
	if bc := update.BusinessConnection; bc != nil {
		owners[bc.ID] = bc.User.ID
	}

	bm := update.BusinessMessage
	if bm == nil || bm.From == nil {
		continue
	}

	// Skip messages sent on behalf of the business account by a bot.
	if bm.SenderBusinessBot != nil {
		continue
	}

	ownerID, ok := owners[bm.BusinessConnectionID]
	if !ok {
		bc, err := bot.GetBusinessConnection(tgbotapi.GetBusinessConnectionConfig{
			BusinessConnectionID: bm.BusinessConnectionID,
		})
		if err != nil {
			log.Println("failed to get business connection:", err)
			continue
		}
		ownerID = bc.User.ID
		owners[bm.BusinessConnectionID] = ownerID
	}

	// Skip the account owner's own outgoing messages.
	if bm.From.ID == ownerID {
		continue
	}

	log.Printf("Received business message from [%s]: %s", bm.From.UserName, bm.Text)

	// Reply on behalf of the business account.
	reply := tgbotapi.NewMessage(bm.Chat.ID, "Thanks for contacting our business! We will get back to you shortly.")
	reply.BusinessConnectionID = bm.BusinessConnectionID
	reply.ReplyParameters = &tgbotapi.ReplyParameters{
		MessageID: bm.MessageID,
	}

	if _, err := bot.Send(reply); err != nil {
		log.Println("failed to reply:", err)
	}
}
```

## Inspecting Business Connections

You can also retrieve connection details at any time using `GetBusinessConnection`:

```go
bc, err := bot.GetBusinessConnection(tgbotapi.GetBusinessConnectionConfig{
	BusinessConnectionID: connectionID,
})
if err != nil {
	log.Panic(err)
}

log.Printf("Connection %s is active: %t", bc.ID, bc.IsEnabled)
```
