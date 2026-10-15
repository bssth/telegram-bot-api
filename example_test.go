package tgbotapi_test

import (
	"log"
	"net/http"

	tgbotapi "github.com/bssth/telegram-bot-api/v6"
)

// ExampleNewBotAPI demonstrates creating a bot and receiving updates
// via long-polling with an echo bot that replies to each message.
func ExampleNewBotAPI() {
	bot, err := tgbotapi.NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
			msg.ReplyParameters = &tgbotapi.ReplyParameters{
				MessageID: update.Message.MessageID,
			}

			if _, err := bot.Send(msg); err != nil {
				log.Println("Failed to send message:", err)
			}
		}
	}
}

// ExampleBotAPI_ListenForWebhook demonstrates setting a webhook and
// handling incoming updates via an HTTPS server.
func ExampleBotAPI_ListenForWebhook() {
	bot, err := tgbotapi.NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	wh, err := tgbotapi.NewWebhookWithCert("https://www.example.com:8443/"+bot.Token, tgbotapi.FilePath("cert.pem"))
	if err != nil {
		log.Fatal(err)
	}

	if _, err := bot.Request(wh); err != nil {
		log.Fatal(err)
	}

	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Fatal(err)
	}

	if info.LastErrorDate != 0 {
		log.Printf("Telegram callback failed: %s", info.LastErrorMessage)
	}

	updates := bot.ListenForWebhook("/" + bot.Token)
	go http.ListenAndServeTLS("0.0.0.0:8443", "cert.pem", "key.pem", nil)

	for update := range updates {
		log.Printf("%+v\n", update)
	}
}

// ExampleNewInlineKeyboardMarkup demonstrates creating an inline keyboard
// with URL and callback data buttons and attaching it to a message.
func ExampleNewInlineKeyboardMarkup() {
	bot, err := tgbotapi.NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		log.Panic(err)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Documentation", "https://bssth.github.io/telegram-bot-api/"),
			tgbotapi.NewInlineKeyboardButtonData("Help", "help"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Option 1", "opt_1"),
			tgbotapi.NewInlineKeyboardButtonData("Option 2", "opt_2"),
		),
	)

	msg := tgbotapi.NewMessage(12345, "Please choose an option:")
	msg.ReplyMarkup = keyboard

	if _, err := bot.Send(msg); err != nil {
		log.Panic(err)
	}
}

// ExampleNewMediaGroup demonstrates sending an album of photos.
func ExampleNewMediaGroup() {
	bot, err := tgbotapi.NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		log.Panic(err)
	}

	photo1 := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath("tests/image.jpg"))
	photo2 := tgbotapi.NewInputMediaPhoto(tgbotapi.FileURL("https://example.com/photo.jpg"))
	photo2.Caption = "Photo from URL"

	mediaGroup := tgbotapi.NewMediaGroup(12345, []interface{}{
		photo1,
		photo2,
	})

	if _, err := bot.SendMediaGroup(mediaGroup); err != nil {
		log.Panic(err)
	}
}

// ExampleNewPoll demonstrates creating a poll with option media.
func ExampleNewPoll() {
	bot, err := tgbotapi.NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		log.Panic(err)
	}

	poll := tgbotapi.NewPoll(12345, "Which programming language do you prefer?", "Go", "Rust")
	poll.Options[0].Media = tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath("tests/image.jpg"))
	poll.Options[1].Media = tgbotapi.NewInputMediaPhoto(tgbotapi.FileURL("https://example.com/rust.png"))

	if _, err := bot.Send(poll); err != nil {
		log.Panic(err)
	}
}

// ExampleNewRichMessage demonstrates sending a rich message built from
// structured layout blocks.
func ExampleNewRichMessage() {
	bot, err := tgbotapi.NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		log.Panic(err)
	}

	richMsg := tgbotapi.NewRichMessage(12345, tgbotapi.InputRichMessage{
		Blocks: []tgbotapi.InputRichBlock{
			{
				Type: tgbotapi.RichBlockTypeHeading,
				Size: 1,
				Text: &tgbotapi.RichText{IsPlain: true, PlainText: "Release Notes"},
			},
			{
				Type: tgbotapi.RichBlockTypeParagraph,
				Text: &tgbotapi.RichText{IsPlain: true, PlainText: "Bot API 10.3 support is now available."},
			},
		},
	})

	if _, err := bot.Send(richMsg); err != nil {
		log.Panic(err)
	}
}

// ExampleReplyParameters demonstrates replying to a message using
// structured ReplyParameters.
func ExampleReplyParameters() {
	bot, err := tgbotapi.NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		log.Panic(err)
	}

	msg := tgbotapi.NewMessage(12345, "This is a reply to message 42")
	msg.ReplyParameters = &tgbotapi.ReplyParameters{
		MessageID:                42,
		AllowSendingWithoutReply: true,
	}

	if _, err := bot.Send(msg); err != nil {
		log.Panic(err)
	}
}

// ExampleLinkPreviewOptions demonstrates customizing link preview behavior.
func ExampleLinkPreviewOptions() {
	bot, err := tgbotapi.NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		log.Panic(err)
	}

	msg := tgbotapi.NewMessage(12345, "Check our documentation at https://bssth.github.io/telegram-bot-api/")
	msg.LinkPreviewOptions = &tgbotapi.LinkPreviewOptions{
		// PreferLargeMedia is ignored unless URL is set explicitly.
		URL:              "https://bssth.github.io/telegram-bot-api/",
		PreferLargeMedia: true,
		ShowAboveText:    true,
	}

	if _, err := bot.Send(msg); err != nil {
		log.Panic(err)
	}
}
