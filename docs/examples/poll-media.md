# Poll Media

Telegram Bot API 10.0+ allows attaching media to poll questions, poll options, and quiz explanations.

## Attaching Media to Poll Options

Create a poll with `NewPoll` and assign media to individual options using the `Media` field on `InputPollOption`:

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

## Attaching Media to the Question and Explanation

You can also attach media to the poll itself or the quiz explanation:

```go
poll := tgbotapi.NewPoll(12345, "Identify this building:", "Eiffel Tower", "Empire State")
poll.Type = "quiz"
poll.CorrectOptionIDs = []int{0}

// Attach a photo to the question.
poll.Media = tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath("tests/image.jpg"))

// Attach media to the explanation shown after an answer is chosen.
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
