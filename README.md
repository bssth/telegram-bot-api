# Golang bindings for the Telegram Bot API

[![Go Reference](https://pkg.go.dev/badge/github.com/bssth/telegram-bot-api/v6.svg)](https://pkg.go.dev/github.com/bssth/telegram-bot-api/v6)
[![Test](https://github.com/bssth/telegram-bot-api/actions/workflows/test.yml/badge.svg)](https://github.com/bssth/telegram-bot-api/actions/workflows/test.yml)
[![Bot API](https://img.shields.io/badge/Bot%20API-10.3-blue.svg)](https://core.telegram.org/bots/api-changelog)

> **This is the maintained continuation of
> [`go-telegram-bot-api/telegram-bot-api`](https://github.com/go-telegram-bot-api/telegram-bot-api).**
> The original project stopped at Bot API 6.0 (April 2022) and no longer
> accepts changes. This fork supports **Bot API 10.3** (August 24, 2026) and
> follows the [official changelog](https://core.telegram.org/bots/api-changelog).

All methods are fairly self-explanatory, and reading the
[godoc](https://pkg.go.dev/github.com/bssth/telegram-bot-api/v6) page should
explain everything. If something isn't clear, open an
[issue](https://github.com/bssth/telegram-bot-api/issues) or submit a pull
request.

The scope of this project is just to provide a wrapper around the API without
any additional features. There are other projects for creating something with
plugins and command handlers without having to design all that yourself.

More tutorials and high-level information live in the [`docs`](./docs)
directory.

## Installing

```sh
go get github.com/bssth/telegram-bot-api/v6@latest
```

```go
import tgbotapi "github.com/bssth/telegram-bot-api/v6"
```

Go 1.24 or newer is required.

The major version is **v6**: the API is not source-compatible with upstream
v5 (see below), so the fork starts a new major version instead of breaking
code that pins `/v5`.

## Migrating from `go-telegram-bot-api/telegram-bot-api`

1. Change the import path. The package name stays `tgbotapi`, so nothing else
   in your code needs to be renamed:

   ```sh
   grep -rl 'github.com/go-telegram-bot-api/telegram-bot-api/v5' --include='*.go' . \
     | xargs sed -i 's#github.com/go-telegram-bot-api/telegram-bot-api/v5#github.com/bssth/telegram-bot-api/v6#g'
   go get github.com/bssth/telegram-bot-api/v6@latest
   go mod tidy
   ```

   (On macOS use `sed -i ''`.) A `replace` directive in `go.mod` is not
   enough: Go requires a replacement module to declare the same module path
   as the one it replaces.

2. Fix whatever no longer compiles using [**BREAKING.md**](./BREAKING.md). It
   lists every source-incompatible change between upstream v5.5.1 (Bot API
   6.0) and this fork, grouped by topic, with before/after snippets. Most of
   them come from Telegram itself: `Thumb` became `Thumbnail`,
   `ReplyToMessageID` became `ReplyParameters`, `DisableWebPagePreview` became
   `LinkPreviewOptions`, and so on.

`BREAKING.md` is written so it can be handed to a coding assistant together
with the files that use `tgbotapi`; it handles most of the mechanical
rewrites in one pass. Review the diff, run `go build ./...`, done.

## Example

This is a very simple bot that just displays any gotten updates, then replies
it to that chat.

```go
package main

import (
	"log"

	tgbotapi "github.com/bssth/telegram-bot-api/v6"
)

func main() {
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
		if update.Message != nil { // If we got a message
			log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
			msg.ReplyParameters = &tgbotapi.ReplyParameters{
				MessageID: update.Message.MessageID,
			}

			bot.Send(msg)
		}
	}
}
```

If you need to use webhooks, you may use a slightly different method.

```go
package main

import (
	"log"
	"net/http"

	tgbotapi "github.com/bssth/telegram-bot-api/v6"
)

func main() {
	bot, err := tgbotapi.NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	wh, _ := tgbotapi.NewWebhookWithCert("https://www.example.com:8443/"+bot.Token, tgbotapi.FilePath("cert.pem"))

	_, err = bot.Request(wh)
	if err != nil {
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
```

If you need, you may generate a self-signed certificate, as this requires
HTTPS / TLS. The above example tells Telegram that this is your certificate
and that it should be trusted, even though it is not properly signed.

    openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 3560 -subj "//O=Org\CN=Test" -nodes

Now that [Let's Encrypt](https://letsencrypt.org) is available, you may wish
to generate your free TLS certificate there.

---

## What changed since upstream

### Bot API coverage

Every Bot API version from **6.1 through 10.3** is supported: all types,
fields, methods and parameters of the current specification are present.
Highlights of the last releases:

- **10.3** — rich message buttons (`RichMessageButton`, the "buttons",
  "document" and "expandable_blockquote" blocks), structured ephemeral send
  parameters (`EphemeralMessageParameters`), disabled buttons and force-reply
  keyboards (`DisabledButton`, `InlineKeyboardButton.Disabled`), stoppable
  drafts (`CanStop` / `KeepOnStop`, `Update.StoppedMessageGeneration`),
  welcome message rights, `CommunityChatJoined`, gift text fields.
- **10.2** — block-structured rich messages (`InputRichBlock`), explicit media
  for rich messages (`InputRichMessageMedia`, `InputMediaVoiceNote`),
  ephemeral messages (`ReplyParameters.EphemeralMessageID`, the
  `EditEphemeralMessage*` and `DeleteEphemeralMessage` configs,
  `Message.ReceiverUser`), communities, payment subscription updates.
- **10.1** — rich messages (`NewRichMessage`, `SendRichMessageDraftConfig`,
  `RichMessage` / `RichText` / `RichBlock`), join request queries
  (`AnswerChatJoinRequestQueryConfig`, `SendChatJoinRequestWebAppConfig`),
  poll links (`InputMediaLink`).
- **10.0** — guest mode (`AnswerGuestQuery`, `Update.GuestMessage`), live
  photos (`NewLivePhoto`, `InputMediaLivePhoto`), poll media for questions,
  options and explanations, reaction administration, managed bot access
  settings.
- **9.x** — managed bots, checklists, suggested posts, direct messages in
  channels, gifts and Telegram Stars, business accounts, stories.
- **7.x – 8.x** — replies 2.0 (`ReplyParameters`), link preview options,
  reactions, boosts, giveaways, business connections, paid media, Mini App
  improvements, and much more.

See the git history for the per-version commits; each `Full support of API X`
commit message lists what that version added.

### Keeping up with the Bot API

`internal/cmd/specdiff` compares the code with the machine-readable
[Bot API specification](https://github.com/PaulSonOfLars/telegram-bot-api-spec)
and lists every missing type, field, method and parameter:

```sh
go run ./internal/cmd/specdiff       # add -v to also list non-spec extras
```

The [Bot API spec](./.github/workflows/bot-api-spec.yml) workflow runs it
weekly, so a new Bot API release shows up as a failed run.

### Credits

The bulk of the Bot API 6.1 → 10.3 work comes from
[go-telegram-bot-api/telegram-bot-api#794](https://github.com/go-telegram-bot-api/telegram-bot-api/pull/794)
by [@kirugan](https://github.com/kirugan), which was never merged upstream.
This fork merges it with authorship preserved and builds on top of it with a
spec-driven audit (see below), plus ideas from other unmerged upstream pull
requests.

### Fixes on top of the upstream pull request

- Placeholder types that only kept raw JSON (`VideoQuality`, `UserRating`,
  `UserProfileAudios`, `GiftBackground`, `UniqueGiftColors`) are real structs.
- `OwnedGift` decodes unique gifts correctly; `repostStory`,
  `getUserGifts`, `getChatGifts`, `setStickerSetThumbnail`, `setGameScore`
  (`force`) and `sendDocument` (`caption_entities`) send the parameters the
  Bot API expects; `suggested_post_parameters` is supported by every send
  method; `setPassportDataErrors` is available.
- Files nested in polls and rich messages are uploaded via `attach://`
  instead of being serialized into the JSON.
- `Update.FromChat` / `SentFrom` cover all update kinds and no longer panic
  on callback queries from inline messages.
- Transport errors no longer leak the bot token, and `GetFileDirectURL`
  honours `SetFileEndpoint`.

### Upstream issues fixed

Real bugs and gaps reported against the original repository that are now
closed in this fork:

- [#781](https://github.com/go-telegram-bot-api/telegram-bot-api/issues/781)
  / [#745](https://github.com/go-telegram-bot-api/telegram-bot-api/issues/745)
  — `SetGameScoreConfig` serialized the score under the key `scrore`; game
  scores never reached Telegram.
- [#628](https://github.com/go-telegram-bot-api/telegram-bot-api/issues/628)
  — `GetUpdatesChan` logged raw `http.Post` errors, which embed the full
  request URL including the bot token. The token is now redacted before
  logging.
- [#683](https://github.com/go-telegram-bot-api/telegram-bot-api/issues/683)
  — `FileEndpoint` was a hard-coded package constant, so
  `GetFileDirectURL` still pointed at `api.telegram.org` when you were
  running a local Bot API server. There is now a `SetFileEndpoint` /
  `FileLink` pair and the field is per-bot.
- [#740](https://github.com/go-telegram-bot-api/telegram-bot-api/issues/740)
  — `InlineConfig.CacheTime = 0` was silently dropped by `AddNonZero`, so
  Telegram applied its 300s default instead of disabling the cache.
  `cache_time` is now serialized unconditionally.
- [#639](https://github.com/go-telegram-bot-api/telegram-bot-api/issues/639)
  / [#705](https://github.com/go-telegram-bot-api/telegram-bot-api/issues/705)
  — `Send()` tried to unmarshal the bare `true` returned by methods like
  `banChatMember` / `setChatTitle` / `sendChatAction`, producing
  `json: cannot unmarshal bool into Message`. It now returns a zero
  `Message, nil` for those shapes. (Prefer `Request` for methods whose
  documented return type is not a `Message`.)
- [#624](https://github.com/go-telegram-bot-api/telegram-bot-api/issues/624)
  — README webhook example passed `"cert.pem"` as a `string` to
  `NewWebhookWithCert`, which takes a `RequestFileData`. Example now uses
  `tgbotapi.FilePath("cert.pem")`.

### Drive-by improvements

Small enhancements that didn't correspond to a filed issue but were worth
doing while the code was already open:

- **Multipart upload name collision fix.** `prepareInputMediaFile` was using
  `file-%d` for *both* the main media and the thumbnail on `InputMediaAudio`
  / `InputMediaDocument`. The two multipart fields collided in the same form
  body; any audio or document upload with a thumbnail probably didn't work on
  the wire. Thumbnails now use `file-%d-thumbnail`, matching the existing
  `InputMediaVideo` pattern.
- **`closeBody` drains response body before close.** `json.Decoder` can stop
  short of EOF, and `net/http` will discard the underlying TCP connection if
  the body isn't fully consumed — no keep-alive reuse. The fork drains the
  remainder before `Close()`.
- **`Params.AddAny(key, value any) error`.** Replaces the older
  `AddInterface` — same behavior, but uses `any` and has a clearer name.
  `AddInterface` is kept as an alias for existing callers.
- **`Params.AddFirstValid` errors are propagated.** The upstream pattern
  ignored its return value inside `params()` methods, so JSON marshaling
  errors in complex fields were silently swallowed. The fork consistently
  captures and returns it.
- **`json.RawMessage` for JSON-serialized string fields.** Fields the
  Telegram docs describe as "JSON-serialized object" (e.g. `provider_data`
  on `InvoiceConfig` / `InvoiceLinkConfig`) are now `json.RawMessage`
  instead of `string`, so you can hand them an already-marshaled payload
  without double-encoding.
