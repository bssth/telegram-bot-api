# Change Log

## v6.0.0 (unreleased)

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
