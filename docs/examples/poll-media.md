# Poll Media

Telegram Bot API 10.0+ allows attaching media to polls: to the poll description, to individual poll options, and to quiz explanations.

## Attaching Media to Poll Options

Create a poll with `NewPoll` and assign media to individual options using the `Media` field on `InputPollOption`:

```go
package main

import (
	"log"
	"os"

	tgbotapi "github.com/bssth/telegram-bot-api/v7"
)

func main() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil {
		log.Panic(err)
	}

	poll := tgbotapi.NewPoll(12345, "Which logo do you like best?", "Option A", "Option B")

	// Attach a local photo to the first option.
	poll.Options[0].Media = tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath("tests/image.jpg"))

	// Attach a remote image URL to the second option.
	poll.Options[1].Media = tgbotapi.NewInputMediaPhoto(tgbotapi.FileURL("https://example.com/logo-b.png"))

	if _, err := bot.Send(poll); err != nil {
		log.Panic(err)
	}
}
```

## Attaching Media to the Description and Explanation

You can also attach media to the poll description (`Media`) or to the quiz explanation (`ExplanationMedia`):

```go
poll := tgbotapi.NewPoll(12345, "Identify this building:", "Eiffel Tower", "Empire State")
poll.Type = "quiz"
poll.CorrectOptionIDs = []int{0}

// Add a description to the poll and attach a photo to it.
poll.Description = "A famous landmark."
poll.Media = tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath("tests/image.jpg"))

// Attach media to the explanation, which is shown when the user chooses an
// incorrect answer or taps the lamp icon.
poll.Explanation = "This is the Eiffel Tower in Paris."
poll.ExplanationMedia = tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath("tests/image.jpg"))

if _, err := bot.Send(poll); err != nil {
	log.Panic(err)
}
```

## Supported Media Types

The following media types can be attached to poll options:
- `InputMediaAnimation` (`NewInputMediaAnimation`)
- `InputMediaLink` (`NewInputMediaLink`)
- `InputMediaLivePhoto` (`NewInputMediaLivePhoto`)
- `InputMediaLocation` (`NewInputMediaLocation`)
- `InputMediaPhoto` (`NewInputMediaPhoto`)
- `InputMediaSticker` (`NewInputMediaSticker`)
- `InputMediaVenue` (`NewInputMediaVenue`)
- `InputMediaVideo` (`NewInputMediaVideo`)

Local file uploads (`FilePath`, `FileBytes`, `FileReader`) are automatically serialized via `attach://poll-media-%d` and uploaded as multipart form data.
