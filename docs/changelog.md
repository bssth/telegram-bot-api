# Change Log

## v7.0.0

- Module path is now `github.com/bssth/telegram-bot-api/v7`. See the
  v6 → v7 section of [BREAKING.md][breaking] for the full migration guide.
- Add `context.Context` support:
  - `MakeRequestWithContext`, `UploadFilesWithContext`, `RequestWithContext`
    and `SendWithContext`. The existing methods call them with
    `context.Background()`.
  - `GetUpdatesWithContext`, `GetMeWithContext`, `GetFileWithContext` and
    `SendMediaGroupWithContext`.
  - Generic `RequestTyped[T]` and `RequestTypedWithContext[T]`, which decode
    a request's result into `T`.
  - `GetUpdatesChanWithContext`: cancelling the context aborts the running
    long poll (and the wait before a retry) and closes the channel.
  - Cancelling an upload returns the context's error, and the uploaded file
    is closed on every path.
- `StopReceivingUpdates` is idempotent and polling can be restarted on the
  same `BotAPI`. It now stops only pollers that are already running: calling
  it before `GetUpdatesChan` no longer makes that call return a closed
  channel. Use `GetUpdatesChanWithContext` for race-free shutdown, and set
  `UpdateConfig.Offset` to the last update ID plus one when restarting.
- Add optional automatic retry on 429 Too Many Requests:
  - `BotAPI.MaxRetries` (default 0, disabled) retries after the
    `retry_after` Telegram asks for. The wait honours the request context.
  - `BotAPI.MaxRetryAfter` limits how long the bot is willing to wait; a
    longer `retry_after` returns the 429 error immediately.
  - Uploads are retried only when every uploaded file is a `FileBytes` or a
    `FilePath`.
  - `IsTooManyRequests(err)` reports whether an error is a 429 and returns
    its `retry_after`.
- Verify the `X-Telegram-Bot-Api-Secret-Token` header of webhook requests:
  - Set `BotAPI.WebhookSecretToken` to the `SecretToken` passed to
    `setWebhook`. `HandleUpdate` then returns `ErrBadSecretToken`, and
    `ListenForWebhook` / `ListenForWebhookRespReqFormat` answer
    401 Unauthorized, when the header is missing or wrong. Nothing changes
    while the field is empty.
  - Webhook error responses now carry their `Content-Type: application/json`
    header.
- Remove deprecated methods and fields:
  - `BotAPI.GetChatMembersCount`: use `GetChatMemberCount`.
  - `BanChatSenderChatConfig.UntilDate`: no longer part of `banChatSenderChat`.
  - `InlineQueryResultPhoto.MimeType` and `InlineQueryResultCachedSticker.Title`: not part of the Bot API; `NewInlineQueryResultCachedSticker` no longer accepts `title`.
  - The `EphemeralSendParams` alias: use `EphemeralMessageParameters`. The
    field embedded in send configs is renamed with it:
    `MessageConfig{EphemeralSendParams: …}` becomes
    `MessageConfig{EphemeralMessageParameters: …}` and
    `cfg.EphemeralSendParams.X` becomes `cfg.EphemeralMessageParameters.X`;
    promoted access such as `cfg.ReceiverUserID` is unchanged.
  - `Params.AddInterface`: use `AddAny`.
- Documentation:
  - Runnable godoc examples on pkg.go.dev.
  - The documentation book is published at
    <https://bssth.github.io/telegram-bot-api/>, with new pages for rich
    messages, ephemeral messages, poll media and business connections.
- Development:
  - Integration tests read the bot token and target chats from environment
    variables and skip cleanly without them. Once the test bot's secrets are
    configured, CI runs them on pushes to `master`, daily and on demand.
  - The weekly Bot API spec check opens (or updates) a `bot-api-update` issue
    when a new Bot API version adds something the library lacks.
  - `specdiff` gained `-format text|markdown|json`.

## v6.0.0

- Module path is now `github.com/bssth/telegram-bot-api/v6`: the upstream
  repository is no longer maintained, and the new major version reflects
  the source-incompatible changes listed in BREAKING.md.
- Support every Bot API version from 6.1 to 10.3, based on the unmerged
  upstream pull request
  [#794](https://github.com/go-telegram-bot-api/telegram-bot-api/pull/794).
- Audit against the machine-readable Bot API 10.3 specification: all types,
  fields, methods and parameters are present.
- Upload files nested in polls and rich messages via `attach://`.
- Redact the bot token from transport errors.
- Add `internal/cmd/specdiff` and a weekly workflow that report gaps against
  the latest Bot API specification.
- Require Go 1.24.
- See [BREAKING.md][breaking] for source-incompatible changes.

[breaking]: https://github.com/bssth/telegram-bot-api/blob/master/BREAKING.md

## v5.4.0

- Remove all methods that return `(APIResponse, error)`.
  - Use the `Request` method instead.
  - For more information, see [Library Structure][library-structure].
- Remove all `New*Upload` and `New*Share` methods, replace with `New*`.
  - Use different [file types][files] to specify if upload or share.
- Rename `UploadFile` to `UploadFiles`, accept `[]RequestFile` instead of a
  single fieldname and file.
- Fix methods returning `APIResponse` and errors to always use pointers.
- Update user IDs to `int64` because of Bot API changes.
- Add missing Bot API features.

[library-structure]: ./getting-started/library-structure.md#methods
[files]: ./getting-started/files.md
