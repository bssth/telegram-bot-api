# Rich Messages

Rich messages (`sendRichMessage`) let bots send structured content formatted with Markdown, HTML, or layout blocks. Rich messages were introduced in Telegram Bot API 10.1; Bot API 10.2 added block-structured layouts (`InputRichBlock`) and explicit embedded media (`InputRichMessageMedia`), and Bot API 10.3 added interactive buttons (`RichMessageButton`).

## Creating a Rich Message

Use the `NewRichMessage` helper, which accepts a chat ID and an `InputRichMessage`. Exactly one of `Markdown`, `HTML`, or `Blocks` must be provided.

### Markdown with Embedded Media

You can write Markdown text and reference embedded media using `tg://` URI schemes mapped to entries in the `Media` slice. Local files (`FilePath`), remote URLs (`FileURL`), or Telegram file IDs (`FileID`) are automatically handled:

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

	richMsg := tgbotapi.NewRichMessage(12345, tgbotapi.InputRichMessage{
		Markdown: "# Product Announcement\n\nTake a look at our new feature:\n![Screenshot](tg://photo?id=shot1)\n\nLearn more at https://example.com.",
		Media: []tgbotapi.InputRichMessageMedia{
			{
				ID:    "shot1",
				Media: tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath("tests/image.jpg")),
			},
		},
	})

	if _, err := bot.Send(richMsg); err != nil {
		log.Panic(err)
	}
}
```

### Block-Structured Layout

When you need fine-grained control over layout and elements, build the message using `InputRichBlock` elements:

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

	richMsg := tgbotapi.NewRichMessage(12345, tgbotapi.InputRichMessage{
		Blocks: []tgbotapi.InputRichBlock{
			{
				Type: tgbotapi.RichBlockTypeHeading,
				Size: 1, // heading size, 1-6; 1 is the largest
				Text: &tgbotapi.RichText{IsPlain: true, PlainText: "Release Notes"},
			},
			{
				Type: tgbotapi.RichBlockTypeParagraph,
				Text: &tgbotapi.RichText{IsPlain: true, PlainText: "Bot API 10.3 support is now available in telegram-bot-api."},
			},
			{
				Type: tgbotapi.RichBlockTypeDivider,
			},
			{
				Type:     tgbotapi.RichBlockTypePre,
				Text:     &tgbotapi.RichText{IsPlain: true, PlainText: "go get github.com/bssth/telegram-bot-api/v6@latest"},
				Language: "sh",
			},
		},
	})

	if _, err := bot.Send(richMsg); err != nil {
		log.Panic(err)
	}
}
```

The library automatically uploads files referenced in blocks or media lists via `attach://` multipart form entries.
