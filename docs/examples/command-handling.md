# Command Handling

This is a simple example of changing behavior based on a provided command.

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

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message updates
			continue
		}

		if !update.Message.IsCommand() { // ignore any non-command Messages
			continue
		}

		// Create a new MessageConfig. We don't have text yet,
		// so we leave it empty.
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		msg.ReplyParameters = &tgbotapi.ReplyParameters{
			MessageID: update.Message.MessageID,
		}

		// Extract the command from the Message.
		switch update.Message.Command() {
		case "help":
			msg.Text = "I understand /sayhi, /status, and /docs."
		case "docs":
			msg.Text = "Documentation: https://bssth.github.io/telegram-bot-api/"
			// Show a large preview of the documentation link above the text.
			// PreferLargeMedia is ignored unless URL is set explicitly.
			msg.LinkPreviewOptions = &tgbotapi.LinkPreviewOptions{
				URL:              "https://bssth.github.io/telegram-bot-api/",
				PreferLargeMedia: true,
				ShowAboveText:    true,
			}
		case "sayhi":
			msg.Text = "Hi :)"
		case "status":
			msg.Text = "I'm ok."
		default:
			msg.Text = "I don't know that command"
		}

		if _, err := bot.Send(msg); err != nil {
			log.Panic(err)
		}
	}
}
```
