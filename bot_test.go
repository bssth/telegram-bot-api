package tgbotapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type testLogger struct {
	t *testing.T
}

func (t testLogger) Println(v ...interface{}) {
	t.t.Log(v...)
}

func (t testLogger) Printf(format string, v ...interface{}) {
	t.t.Logf(format, v...)
}

func getBot(t *testing.T) (*BotAPI, error) {
	t.Helper()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		t.Skip("skipping integration test: TELEGRAM_BOT_TOKEN not set")
		return nil, nil
	}

	var bot *BotAPI
	var err error
	if endpoint := os.Getenv("TELEGRAM_API_ENDPOINT"); endpoint != "" {
		bot, err = NewBotAPIWithAPIEndpoint(token, endpoint)
	} else {
		bot, err = NewBotAPI(token)
	}
	if err != nil {
		t.Fatalf("NewBotAPI failed: %v", err)
		return nil, err
	}

	// Debug dumps raw API responses (including user names) into the test
	// output, which ends up in public CI logs, so it is opt-in.
	if os.Getenv("TELEGRAM_TEST_DEBUG") != "" {
		bot.Debug = true
	}

	// Route package logging into this test and put the previous logger back
	// afterwards, so later tests don't log into a finished test.
	prevLogger := log
	SetLogger(testLogger{t})
	t.Cleanup(func() { SetLogger(prevLogger) })

	return bot, nil
}

func getChatID(t *testing.T) int64 {
	t.Helper()

	val := os.Getenv("TELEGRAM_TEST_CHAT_ID")
	if val == "" {
		t.Skip("skipping integration test: TELEGRAM_TEST_CHAT_ID not set")
	}

	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		t.Fatalf("invalid TELEGRAM_TEST_CHAT_ID %q: %v", val, err)
	}

	return id
}

func getChannel(t *testing.T) string {
	t.Helper()

	val := os.Getenv("TELEGRAM_TEST_CHANNEL")
	if val == "" {
		t.Skip("skipping integration test: TELEGRAM_TEST_CHANNEL not set")
	}

	return val
}

func getSupergroupChatID(t *testing.T) int64 {
	t.Helper()

	val := os.Getenv("TELEGRAM_TEST_SUPERGROUP_ID")
	if val == "" {
		t.Skip("skipping integration test: TELEGRAM_TEST_SUPERGROUP_ID not set")
	}

	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		t.Fatalf("invalid TELEGRAM_TEST_SUPERGROUP_ID %q: %v", val, err)
	}

	return id
}

func getOptionalSupergroupChatID(t *testing.T) int64 {
	t.Helper()

	val := os.Getenv("TELEGRAM_TEST_SUPERGROUP_ID")
	if val == "" {
		return 0
	}

	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		t.Fatalf("invalid TELEGRAM_TEST_SUPERGROUP_ID %q: %v", val, err)
	}

	return id
}

var (
	cachedFileIDsMu sync.Mutex
	cachedFileIDs   = make(map[string]string)
)

func getOrUploadFileID(t *testing.T, kind string, upload func() (string, error)) string {
	t.Helper()

	cachedFileIDsMu.Lock()
	defer cachedFileIDsMu.Unlock()

	if id, ok := cachedFileIDs[kind]; ok {
		return id
	}

	envVar := "TELEGRAM_TEST_" + strings.ToUpper(kind) + "_FILE_ID"
	if id := os.Getenv(envVar); id != "" {
		cachedFileIDs[kind] = id
		return id
	}

	id, err := upload()
	if err != nil {
		t.Fatalf("failed to obtain file ID for %s: %v", kind, err)
	}

	cachedFileIDs[kind] = id
	return id
}

func getExistingPhotoFileID(t *testing.T, bot *BotAPI, chatID int64) string {
	t.Helper()
	return getOrUploadFileID(t, "photo", func() (string, error) {
		msg, err := bot.Send(NewPhoto(chatID, FilePath("tests/image.jpg")))
		if err != nil {
			return "", err
		}
		if len(msg.Photo) == 0 {
			return "", errors.New("sent photo message has no photo items")
		}
		return msg.Photo[len(msg.Photo)-1].FileID, nil
	})
}

func getExistingDocumentFileID(t *testing.T, bot *BotAPI, chatID int64) string {
	t.Helper()
	return getOrUploadFileID(t, "document", func() (string, error) {
		msg, err := bot.Send(NewDocument(chatID, FilePath("tests/image.jpg")))
		if err != nil {
			return "", err
		}
		if msg.Document == nil {
			return "", errors.New("sent document message has no document")
		}
		return msg.Document.FileID, nil
	})
}

func getExistingAudioFileID(t *testing.T, bot *BotAPI, chatID int64) string {
	t.Helper()
	return getOrUploadFileID(t, "audio", func() (string, error) {
		msg, err := bot.Send(NewAudio(chatID, FilePath("tests/audio.mp3")))
		if err != nil {
			return "", err
		}
		if msg.Audio == nil {
			return "", errors.New("sent audio message has no audio")
		}
		return msg.Audio.FileID, nil
	})
}

func getExistingVoiceFileID(t *testing.T, bot *BotAPI, chatID int64) string {
	t.Helper()
	return getOrUploadFileID(t, "voice", func() (string, error) {
		msg, err := bot.Send(NewVoice(chatID, FilePath("tests/voice.ogg")))
		if err != nil {
			return "", err
		}
		if msg.Voice == nil {
			return "", errors.New("sent voice message has no voice")
		}
		return msg.Voice.FileID, nil
	})
}

func getExistingVideoFileID(t *testing.T, bot *BotAPI, chatID int64) string {
	t.Helper()
	return getOrUploadFileID(t, "video", func() (string, error) {
		msg, err := bot.Send(NewVideo(chatID, FilePath("tests/video.mp4")))
		if err != nil {
			return "", err
		}
		if msg.Video == nil {
			return "", errors.New("sent video message has no video")
		}
		return msg.Video.FileID, nil
	})
}

func getExistingVideoNoteFileID(t *testing.T, bot *BotAPI, chatID int64) string {
	t.Helper()
	return getOrUploadFileID(t, "videonote", func() (string, error) {
		msg, err := bot.Send(NewVideoNote(chatID, 240, FilePath("tests/videonote.mp4")))
		if err != nil {
			return "", err
		}
		if msg.VideoNote == nil {
			return "", errors.New("sent video note message has no video note")
		}
		return msg.VideoNote.FileID, nil
	})
}

func getExistingStickerFileID(t *testing.T, bot *BotAPI, chatID int64) string {
	t.Helper()
	return getOrUploadFileID(t, "sticker", func() (string, error) {
		msg, err := bot.Send(NewSticker(chatID, FilePath("tests/sticker.webp")))
		if err != nil {
			return "", err
		}
		if msg.Sticker == nil {
			return "", errors.New("sent sticker message has no sticker")
		}
		return msg.Sticker.FileID, nil
	})
}

func TestNewBotAPI_notoken(t *testing.T) {
	_, err := NewBotAPI("")

	if err == nil {
		t.Error(err)
	}
}

func TestTestHelpersWithFakeServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/getMe") {
			w.Write([]byte(`{"ok":true,"result":{"id":12345,"is_bot":true,"first_name":"TestBot","username":"test_bot"}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	t.Setenv("TELEGRAM_BOT_TOKEN", "123456:ABC-DEF")
	t.Setenv("TELEGRAM_API_ENDPOINT", srv.URL+"/bot%s/%s")
	t.Setenv("TELEGRAM_TEST_CHAT_ID", "987654321")
	t.Setenv("TELEGRAM_TEST_CHANNEL", "@mychannel")
	t.Setenv("TELEGRAM_TEST_SUPERGROUP_ID", "-1001234567890")
	t.Setenv("TELEGRAM_TEST_DEBUG", "")

	prevLogger := log
	t.Run("getBot", func(t *testing.T) {
		bot, err := getBot(t)
		if err != nil {
			t.Fatalf("getBot returned error: %v", err)
		}
		if bot.Self.UserName != "test_bot" {
			t.Fatalf("expected bot username test_bot, got %s", bot.Self.UserName)
		}
		if bot.Debug {
			t.Error("expected Debug to be off when TELEGRAM_TEST_DEBUG is not set")
		}
	})
	if log != prevLogger {
		t.Error("getBot did not restore the package logger after the test finished")
	}

	if chatID := getChatID(t); chatID != 987654321 {
		t.Fatalf("expected chat ID 987654321, got %d", chatID)
	}

	if channel := getChannel(t); channel != "@mychannel" {
		t.Fatalf("expected channel @mychannel, got %s", channel)
	}

	if supergroupID := getSupergroupChatID(t); supergroupID != -1001234567890 {
		t.Fatalf("expected supergroup ID -1001234567890, got %d", supergroupID)
	}

	if optionalSupergroup := getOptionalSupergroupChatID(t); optionalSupergroup != -1001234567890 {
		t.Fatalf("expected optional supergroup ID -1001234567890, got %d", optionalSupergroup)
	}
}

func TestGetUpdates(t *testing.T) {
	bot, _ := getBot(t)

	u := NewUpdate(0)

	_, err := bot.GetUpdates(u)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithMessage(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewMessage(chatID, "A test message from the test library in telegram-bot-api")
	msg.ParseMode = ModeMarkdown
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithMessageReply(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	baseMsg := NewMessage(chatID, "A test message from the test library in telegram-bot-api")
	sent, err := bot.Send(baseMsg)
	if err != nil {
		t.Fatal(err)
	}

	msg := NewMessage(chatID, "A test message from the test library in telegram-bot-api")
	msg.ReplyParameters = &ReplyParameters{MessageID: sent.MessageID}
	_, err = bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithMessageForward(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	baseMsg := NewMessage(chatID, "A test message to forward")
	sent, err := bot.Send(baseMsg)
	if err != nil {
		t.Fatal(err)
	}

	msg := NewForward(chatID, chatID, sent.MessageID)
	_, err = bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestCopyMessage(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)
	targetChatID := getOptionalSupergroupChatID(t)
	if targetChatID == 0 {
		targetChatID = chatID
	}

	msg := NewMessage(chatID, "A test message from the test library in telegram-bot-api")
	message, err := bot.Send(msg)
	if err != nil {
		t.Fatal(err)
	}

	copyMessageConfig := NewCopyMessage(targetChatID, message.Chat.ID, message.MessageID)
	messageID, err := bot.CopyMessage(copyMessageConfig)
	if err != nil {
		t.Error(err)
	}

	if targetChatID == message.Chat.ID && messageID.MessageID == message.MessageID {
		t.Error("copied message ID was the same as original message")
	}
}

func TestSendWithNewPhoto(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewPhoto(chatID, FilePath("tests/image.jpg"))
	msg.Caption = "Test"
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithNewPhotoWithFileBytes(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	data, _ := os.ReadFile("tests/image.jpg")
	b := FileBytes{Name: "image.jpg", Bytes: data}

	msg := NewPhoto(chatID, b)
	msg.Caption = "Test"
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithNewPhotoWithFileReader(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	f, _ := os.Open("tests/image.jpg")
	reader := FileReader{Name: "image.jpg", Reader: f}

	msg := NewPhoto(chatID, reader)
	msg.Caption = "Test"
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithNewPhotoReply(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	baseMsg := NewMessage(chatID, "A test message for photo reply")
	sent, err := bot.Send(baseMsg)
	if err != nil {
		t.Fatal(err)
	}

	msg := NewPhoto(chatID, FilePath("tests/image.jpg"))
	msg.ReplyParameters = &ReplyParameters{MessageID: sent.MessageID}

	_, err = bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendNewPhotoToChannel(t *testing.T) {
	bot, _ := getBot(t)
	channel := getChannel(t)

	msg := NewPhotoToChannel(channel, FilePath("tests/image.jpg"))
	msg.Caption = "Test"
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
		t.Fail()
	}
}

func TestSendNewPhotoToChannelFileBytes(t *testing.T) {
	bot, _ := getBot(t)
	channel := getChannel(t)

	data, _ := os.ReadFile("tests/image.jpg")
	b := FileBytes{Name: "image.jpg", Bytes: data}

	msg := NewPhotoToChannel(channel, b)
	msg.Caption = "Test"
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
		t.Fail()
	}
}

func TestSendNewPhotoToChannelFileReader(t *testing.T) {
	bot, _ := getBot(t)
	channel := getChannel(t)

	f, _ := os.Open("tests/image.jpg")
	reader := FileReader{Name: "image.jpg", Reader: f}

	msg := NewPhotoToChannel(channel, reader)
	msg.Caption = "Test"
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
		t.Fail()
	}
}

func TestSendWithExistingPhoto(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)
	fileID := getExistingPhotoFileID(t, bot, chatID)

	msg := NewPhoto(chatID, FileID(fileID))
	msg.Caption = "Test"
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithNewDocument(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewDocument(chatID, FilePath("tests/image.jpg"))
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithNewDocumentAndThumbnail(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewDocument(chatID, FilePath("tests/voice.ogg"))
	msg.Thumbnail = FilePath("tests/image.jpg")
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithExistingDocument(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)
	fileID := getExistingDocumentFileID(t, bot, chatID)

	msg := NewDocument(chatID, FileID(fileID))
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithNewAudio(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewAudio(chatID, FilePath("tests/audio.mp3"))
	msg.Title = "TEST"
	msg.Duration = 10
	msg.Performer = "TEST"
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithExistingAudio(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)
	fileID := getExistingAudioFileID(t, bot, chatID)

	msg := NewAudio(chatID, FileID(fileID))
	msg.Title = "TEST"
	msg.Duration = 10
	msg.Performer = "TEST"

	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithNewVoice(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewVoice(chatID, FilePath("tests/voice.ogg"))
	msg.Duration = 10
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithExistingVoice(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)
	fileID := getExistingVoiceFileID(t, bot, chatID)

	msg := NewVoice(chatID, FileID(fileID))
	msg.Duration = 10
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithContact(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	contact := NewContact(chatID, "5551234567", "Test")

	if _, err := bot.Send(contact); err != nil {
		t.Error(err)
	}
}

func TestSendWithLocation(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	_, err := bot.Send(NewLocation(chatID, 40, 40))

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithVenue(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	venue := NewVenue(chatID, "A Test Location", "123 Test Street", 40, 40)

	if _, err := bot.Send(venue); err != nil {
		t.Error(err)
	}
}

func TestSendWithNewVideo(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewVideo(chatID, FilePath("tests/video.mp4"))
	msg.Duration = 10
	msg.Caption = "TEST"

	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithExistingVideo(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)
	fileID := getExistingVideoFileID(t, bot, chatID)

	msg := NewVideo(chatID, FileID(fileID))
	msg.Duration = 10
	msg.Caption = "TEST"

	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithNewVideoNote(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewVideoNote(chatID, 240, FilePath("tests/videonote.mp4"))
	msg.Duration = 10

	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithExistingVideoNote(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)
	fileID := getExistingVideoNoteFileID(t, bot, chatID)

	msg := NewVideoNote(chatID, 240, FileID(fileID))
	msg.Duration = 10

	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithNewSticker(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewSticker(chatID, FilePath("tests/sticker.webp"))

	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithExistingSticker(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)
	fileID := getExistingStickerFileID(t, bot, chatID)

	msg := NewSticker(chatID, FileID(fileID))

	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithNewStickerAndKeyboardHide(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewSticker(chatID, FilePath("tests/sticker.webp"))
	msg.ReplyMarkup = ReplyKeyboardRemove{
		RemoveKeyboard: true,
		Selective:      false,
	}
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithExistingStickerAndKeyboardHide(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)
	fileID := getExistingStickerFileID(t, bot, chatID)

	msg := NewSticker(chatID, FileID(fileID))
	msg.ReplyMarkup = ReplyKeyboardRemove{
		RemoveKeyboard: true,
		Selective:      false,
	}

	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
	}
}

func TestSendWithDice(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewDice(chatID)
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
		t.Fail()
	}

}

func TestSendWithDiceWithEmoji(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewDiceWithEmoji(chatID, "🏀")
	_, err := bot.Send(msg)

	if err != nil {
		t.Error(err)
		t.Fail()
	}

}

func TestGetFile(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)
	fileID := getExistingPhotoFileID(t, bot, chatID)

	file := FileConfig{
		FileID: fileID,
	}

	_, err := bot.GetFile(file)

	if err != nil {
		t.Error(err)
	}
}

func TestSendChatConfig(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	_, err := bot.Request(NewChatAction(chatID, ChatTyping))

	if err != nil {
		t.Error(err)
	}
}

// TODO: identify why this isn't working
// func TestSendEditMessage(t *testing.T) {
// 	bot, _ := getBot(t)
// 	chatID := getChatID(t)

// 	msg, err := bot.Send(NewMessage(chatID, "Testing editing."))
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	edit := EditMessageTextConfig{
// 		BaseEdit: BaseEdit{
// 			ChatID:    chatID,
// 			MessageID: msg.MessageID,
// 		},
// 		Text: "Updated text.",
// 	}

// 	_, err = bot.Send(edit)
// 	if err != nil {
// 		t.Error(err)
// 	}
// }

func TestGetUserProfilePhotos(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	_, err := bot.GetUserProfilePhotos(NewUserProfilePhotos(chatID))
	if err != nil {
		t.Error(err)
	}
}

// testWebhookURL is a fixed, unreachable webhook URL. It deliberately does not
// contain the bot token: getWebhookInfo echoes the URL back, and Debug output
// may end up in public CI logs.
const testWebhookURL = "https://example.com/tgbotapi-test/webhook"

// cleanupWebhook removes the webhook when the test finishes, even if it fails
// halfway, so the bot isn't left with a webhook that blocks getUpdates.
func cleanupWebhook(t *testing.T, bot *BotAPI) {
	t.Helper()

	t.Cleanup(func() {
		if _, err := bot.Request(DeleteWebhookConfig{DropPendingUpdates: true}); err != nil {
			t.Errorf("failed to delete webhook: %v", err)
		}
	})
}

func TestSetWebhookWithCert(t *testing.T) {
	bot, _ := getBot(t)

	time.Sleep(time.Second * 2)

	bot.Request(DeleteWebhookConfig{DropPendingUpdates: true})
	cleanupWebhook(t, bot)

	wh, err := NewWebhookWithCert(testWebhookURL, FilePath("tests/cert.pem"))

	if err != nil {
		t.Error(err)
	}
	_, err = bot.Request(wh)

	if err != nil {
		t.Error(err)
	}

	_, err = bot.GetWebhookInfo()

	if err != nil {
		t.Error(err)
	}
}

func TestSetWebhookWithoutCert(t *testing.T) {
	bot, _ := getBot(t)

	time.Sleep(time.Second * 2)

	bot.Request(DeleteWebhookConfig{DropPendingUpdates: true})
	cleanupWebhook(t, bot)

	wh, err := NewWebhook(testWebhookURL)

	if err != nil {
		t.Error(err)
	}

	_, err = bot.Request(wh)

	if err != nil {
		t.Error(err)
	}

	info, err := bot.GetWebhookInfo()

	if err != nil {
		t.Error(err)
	}
	if info.MaxConnections == 0 {
		t.Errorf("Expected maximum connections to be greater than 0")
	}
	if info.LastErrorDate != 0 {
		t.Errorf("failed to set webhook: %s", info.LastErrorMessage)
	}
}

func TestSendWithMediaGroupPhotoVideo(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	cfg := NewMediaGroup(chatID, []interface{}{
		NewInputMediaPhoto(FileURL("https://github.com/go-telegram-bot-api/telegram-bot-api/raw/0a3a1c8716c4cd8d26a262af9f12dcbab7f3f28c/tests/image.jpg")),
		NewInputMediaPhoto(FilePath("tests/image.jpg")),
		NewInputMediaVideo(FilePath("tests/video.mp4")),
	})

	messages, err := bot.SendMediaGroup(cfg)
	if err != nil {
		t.Error(err)
	}

	if messages == nil {
		t.Error("No received messages")
	}

	if len(messages) != len(cfg.Media) {
		t.Errorf("Different number of messages: %d", len(messages))
	}
}

func TestSendWithMediaGroupDocument(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	cfg := NewMediaGroup(chatID, []interface{}{
		NewInputMediaDocument(FileURL("https://github.com/go-telegram-bot-api/telegram-bot-api/raw/0a3a1c8716c4cd8d26a262af9f12dcbab7f3f28c/tests/image.jpg")),
		NewInputMediaDocument(FilePath("tests/image.jpg")),
	})

	messages, err := bot.SendMediaGroup(cfg)
	if err != nil {
		t.Error(err)
	}

	if messages == nil {
		t.Error("No received messages")
	}

	if len(messages) != len(cfg.Media) {
		t.Errorf("Different number of messages: %d", len(messages))
	}
}

func TestSendWithMediaGroupAudio(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	cfg := NewMediaGroup(chatID, []interface{}{
		NewInputMediaAudio(FilePath("tests/audio.mp3")),
		NewInputMediaAudio(FilePath("tests/audio.mp3")),
	})

	messages, err := bot.SendMediaGroup(cfg)
	if err != nil {
		t.Error(err)
	}

	if messages == nil {
		t.Error("No received messages")
	}

	if len(messages) != len(cfg.Media) {
		t.Errorf("Different number of messages: %d", len(messages))
	}
}

func ExampleNewBotAPI() {
	bot, err := NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// Optional: wait for updates and clear them if you don't want to handle
	// a large backlog of old messages
	time.Sleep(time.Millisecond * 500)
	updates.Clear()

	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		msg := NewMessage(update.Message.Chat.ID, update.Message.Text)
		msg.ReplyParameters = &ReplyParameters{MessageID: update.Message.MessageID}

		bot.Send(msg)
	}
}

func ExampleNewWebhook() {
	bot, err := NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	wh, err := NewWebhookWithCert("https://www.google.com:8443/", FilePath("cert.pem"))
	if err != nil {
		panic(err)
	}
	wh.SecretToken = "super-secret-token"
	bot.WebhookSecretToken = wh.SecretToken

	_, err = bot.Request(wh)

	if err != nil {
		panic(err)
	}

	info, err := bot.GetWebhookInfo()

	if err != nil {
		panic(err)
	}

	if info.LastErrorDate != 0 {
		log.Printf("failed to set webhook: %s", info.LastErrorMessage)
	}

	updates := bot.ListenForWebhook("/")
	go http.ListenAndServeTLS("0.0.0.0:8443", "cert.pem", "key.pem", nil)

	for update := range updates {
		log.Printf("%+v\n", update)
	}
}

func ExampleNewWebhookWithCert() {
	bot, err := NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	wh, err := NewWebhookWithCert("https://www.google.com:8443/", FilePath("cert.pem"))

	if err != nil {
		panic(err)
	}
	wh.SecretToken = "super-secret-token"
	bot.WebhookSecretToken = wh.SecretToken

	_, err = bot.Request(wh)
	if err != nil {
		panic(err)
	}
	info, err := bot.GetWebhookInfo()
	if err != nil {
		panic(err)
	}
	if info.LastErrorDate != 0 {
		log.Printf("[Telegram callback failed]%s", info.LastErrorMessage)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		update, err := bot.HandleUpdate(r)
		if err != nil {
			log.Printf("%+v\n", err.Error())
		} else {
			log.Printf("%+v\n", *update)
		}
	})

	go http.ListenAndServeTLS("0.0.0.0:8443", "cert.pem", "key.pem", nil)
}

func ExampleInlineConfig() {
	bot, err := NewBotAPI("MyAwesomeBotToken") // create new bot
	if err != nil {
		panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.InlineQuery == nil { // if no inline query, ignore it
			continue
		}

		article := NewInlineQueryResultArticle(update.InlineQuery.ID, "Echo", update.InlineQuery.Query)
		article.Description = update.InlineQuery.Query

		inlineConf := InlineConfig{
			InlineQueryID: update.InlineQuery.ID,
			IsPersonal:    true,
			CacheTime:     0,
			Results:       []interface{}{article},
		}

		if _, err := bot.Request(inlineConf); err != nil {
			log.Println(err)
		}
	}
}

func TestDeleteMessage(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	msg := NewMessage(chatID, "A test message from the test library in telegram-bot-api")
	msg.ParseMode = ModeMarkdown
	message, err := bot.Send(msg)
	if err != nil {
		t.Fatal(err)
	}

	deleteMessageConfig := DeleteMessageConfig{
		ChatID:    message.Chat.ID,
		MessageID: message.MessageID,
	}
	_, err = bot.Request(deleteMessageConfig)

	if err != nil {
		t.Error(err)
	}
}

func TestPinChatMessage(t *testing.T) {
	bot, _ := getBot(t)
	supergroupID := getSupergroupChatID(t)

	msg := NewMessage(supergroupID, "A test message from the test library in telegram-bot-api")
	msg.ParseMode = ModeMarkdown
	message, err := bot.Send(msg)
	if err != nil {
		t.Fatal(err)
	}

	pinChatMessageConfig := PinChatMessageConfig{
		ChatID:              message.Chat.ID,
		MessageID:           message.MessageID,
		DisableNotification: false,
	}
	_, err = bot.Request(pinChatMessageConfig)

	if err != nil {
		t.Error(err)
	}
}

func TestUnpinChatMessage(t *testing.T) {
	bot, _ := getBot(t)
	supergroupID := getSupergroupChatID(t)

	msg := NewMessage(supergroupID, "A test message from the test library in telegram-bot-api")
	msg.ParseMode = ModeMarkdown
	message, err := bot.Send(msg)
	if err != nil {
		t.Fatal(err)
	}

	// We need pin message to unpin something
	pinChatMessageConfig := PinChatMessageConfig{
		ChatID:              message.Chat.ID,
		MessageID:           message.MessageID,
		DisableNotification: false,
	}

	if _, err := bot.Request(pinChatMessageConfig); err != nil {
		t.Error(err)
	}

	unpinChatMessageConfig := UnpinChatMessageConfig{
		ChatID:    message.Chat.ID,
		MessageID: message.MessageID,
	}

	if _, err := bot.Request(unpinChatMessageConfig); err != nil {
		t.Error(err)
	}
}

func TestUnpinAllChatMessages(t *testing.T) {
	bot, _ := getBot(t)
	supergroupID := getSupergroupChatID(t)

	msg := NewMessage(supergroupID, "A test message from the test library in telegram-bot-api")
	msg.ParseMode = ModeMarkdown
	message, err := bot.Send(msg)
	if err != nil {
		t.Fatal(err)
	}

	pinChatMessageConfig := PinChatMessageConfig{
		ChatID:              message.Chat.ID,
		MessageID:           message.MessageID,
		DisableNotification: true,
	}

	if _, err := bot.Request(pinChatMessageConfig); err != nil {
		t.Error(err)
	}

	unpinAllChatMessagesConfig := UnpinAllChatMessagesConfig{
		ChatID: message.Chat.ID,
	}

	if _, err := bot.Request(unpinAllChatMessagesConfig); err != nil {
		t.Error(err)
	}
}

func TestPolls(t *testing.T) {
	bot, _ := getBot(t)
	supergroupID := getSupergroupChatID(t)

	poll := NewPoll(supergroupID, "Are polls working?", "Yes", "No")

	msg, err := bot.Send(poll)
	if err != nil {
		t.Fatal(err)
	}

	result, err := bot.StopPoll(NewStopPoll(supergroupID, msg.MessageID))
	if err != nil {
		t.Error(err)
	}

	if result.Question != "Are polls working?" {
		t.Error("Poll question did not match")
	}

	if !result.IsClosed {
		t.Error("Poll did not end")
	}

	if result.Options[0].Text != "Yes" || result.Options[0].VoterCount != 0 || result.Options[1].Text != "No" || result.Options[1].VoterCount != 0 {
		t.Error("Poll options were incorrect")
	}
}

func TestSendDice(t *testing.T) {
	bot, _ := getBot(t)
	chatID := getChatID(t)

	dice := NewDice(chatID)

	msg, err := bot.Send(dice)
	if err != nil {
		t.Error("Unable to send dice roll")
	}

	if msg.Dice == nil {
		t.Error("Dice roll was not received")
	}
}

func TestCommands(t *testing.T) {
	bot, _ := getBot(t)

	setCommands := NewSetMyCommands(BotCommand{
		Command:     "test",
		Description: "a test command",
	})

	if _, err := bot.Request(setCommands); err != nil {
		t.Error("Unable to set commands")
	}

	commands, err := bot.GetMyCommands()
	if err != nil {
		t.Error("Unable to get commands")
	}

	if len(commands) != 1 {
		t.Error("Incorrect number of commands returned")
	}

	if commands[0].Command != "test" || commands[0].Description != "a test command" {
		t.Error("Commands were incorrectly set")
	}

	setCommands = NewSetMyCommandsWithScope(NewBotCommandScopeAllPrivateChats(), BotCommand{
		Command:     "private",
		Description: "a private command",
	})

	if _, err := bot.Request(setCommands); err != nil {
		t.Error("Unable to set commands")
	}

	commands, err = bot.GetMyCommandsWithConfig(NewGetMyCommandsWithScope(NewBotCommandScopeAllPrivateChats()))
	if err != nil {
		t.Error("Unable to get commands")
	}

	if len(commands) != 1 {
		t.Error("Incorrect number of commands returned")
	}

	if commands[0].Command != "private" || commands[0].Description != "a private command" {
		t.Error("Commands were incorrectly set")
	}
}

// TODO: figure out why test is failing
//
// func TestEditMessageMedia(t *testing.T) {
// 	bot, _ := getBot(t)
// 	chatID := getChatID(t)

// 	msg := NewPhoto(chatID, FilePath("tests/image.jpg"))
// 	msg.Caption = "Test"
// 	m, err := bot.Send(msg)

// 	if err != nil {
// 		t.Error(err)
// 	}

// 	edit := EditMessageMediaConfig{
// 		BaseEdit: BaseEdit{
// 			ChatID:    chatID,
// 			MessageID: m.MessageID,
// 		},
// 		Media: NewInputMediaVideo(FilePath("tests/video.mp4")),
// 	}

// 	_, err = bot.Request(edit)
// 	if err != nil {
// 		t.Error(err)
// 	}
// }

func TestPrepareInputMediaForParams(t *testing.T) {
	media := []interface{}{
		NewInputMediaPhoto(FilePath("tests/image.jpg")),
		NewInputMediaVideo(FileID("test")),
	}

	prepared := prepareInputMediaForParams(media)

	if media[0].(InputMediaPhoto).Media != FilePath("tests/image.jpg") {
		t.Error("Original media was changed")
	}

	if prepared[0].(InputMediaPhoto).Media != fileAttach("attach://file-0") {
		t.Error("New media was not replaced")
	}

	if prepared[1].(InputMediaVideo).Media != FileID("test") {
		t.Error("Passthrough value was not the same")
	}
}
