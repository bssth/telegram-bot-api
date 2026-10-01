package tgbotapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeBotAPI is a minimal stand-in for the Bot API server. It answers getMe,
// records every other request (including multipart uploads) and replies with
// a successful empty message.
type fakeBotAPI struct {
	mu       sync.Mutex
	requests map[string]*http.Request
	fields   map[string]map[string]string
	files    map[string]map[string]string
}

func newFakeBotAPI(t *testing.T) (*BotAPI, *fakeBotAPI) {
	t.Helper()

	fake := &fakeBotAPI{
		requests: map[string]*http.Request{},
		fields:   map[string]map[string]string{},
		files:    map[string]map[string]string{},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		if method == "getMe" {
			io.WriteString(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Bot","username":"bot"}}`)
			return
		}

		fields, files := map[string]string{}, map[string]string{}
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse multipart: %v", err)
			}
			for k, v := range r.MultipartForm.Value {
				fields[k] = v[0]
			}
			for k, v := range r.MultipartForm.File {
				f, _ := v[0].Open()
				b, _ := io.ReadAll(f)
				files[k] = string(b)
			}
		} else {
			r.ParseForm()
			for k, v := range r.PostForm {
				fields[k] = v[0]
			}
		}

		fake.mu.Lock()
		fake.requests[method] = r
		fake.fields[method] = fields
		fake.files[method] = files
		fake.mu.Unlock()

		io.WriteString(w, `{"ok":true,"result":{"message_id":1,"date":1,"chat":{"id":1,"type":"private"}}}`)
	}))
	t.Cleanup(srv.Close)

	bot, err := NewBotAPIWithAPIEndpoint("TOKEN", srv.URL+"/bot%s/%s")
	if err != nil {
		t.Fatalf("NewBotAPIWithAPIEndpoint: %v", err)
	}

	return bot, fake
}

func TestUploadNestedPollMedia(t *testing.T) {
	bot, fake := newFakeBotAPI(t)

	_, err := bot.Send(SendPollConfig{
		BaseChat: BaseChat{ChatID: 1},
		Question: "Which one?",
		Options: []InputPollOption{
			{Text: "first", Media: NewInputMediaPhoto(FileBytes{Name: "a.jpg", Bytes: []byte("AAA")})},
			{Text: "second"},
		},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if got := fake.files["sendPoll"]["poll-media-0"]; got != "AAA" {
		t.Fatalf("uploaded file = %q, want AAA (files: %v)", got, fake.files["sendPoll"])
	}
	if !strings.Contains(fake.fields["sendPoll"]["options"], "attach://poll-media-0") {
		t.Fatalf("options field = %s", fake.fields["sendPoll"]["options"])
	}
}

func TestUploadRichMessageMedia(t *testing.T) {
	bot, fake := newFakeBotAPI(t)

	_, err := bot.Send(SendRichMessageConfig{
		BaseChat: BaseChat{ChatID: 1},
		RichMessage: &InputRichMessage{
			Markdown: "![](tg://photo?id=p1)",
			Media:    []InputRichMessageMedia{{ID: "p1", Media: NewInputMediaPhoto(FileBytes{Name: "p.jpg", Bytes: []byte("PPP")})}},
		},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if got := fake.files["sendRichMessage"]["rich-media-0"]; got != "PPP" {
		t.Fatalf("uploaded file = %q, want PPP (files: %v)", got, fake.files["sendRichMessage"])
	}
}

func TestSendWithoutUploadUsesPlainForm(t *testing.T) {
	bot, fake := newFakeBotAPI(t)

	_, err := bot.Send(SendPollConfig{
		BaseChat: BaseChat{ChatID: 1},
		Question: "Which one?",
		Options:  []InputPollOption{{Text: "first", Media: NewInputMediaPhoto(FileID("abc"))}, {Text: "second"}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(fake.files["sendPoll"]) != 0 {
		t.Fatalf("unexpected uploads: %v", fake.files["sendPoll"])
	}
	if !strings.Contains(fake.fields["sendPoll"]["options"], `"media":"abc"`) {
		t.Fatalf("options field = %s", fake.fields["sendPoll"]["options"])
	}
}

func TestTransportErrorsDoNotLeakToken(t *testing.T) {
	bot, _ := newFakeBotAPI(t)
	bot.Token = "123456:SECRET"
	bot.SetAPIEndpoint("http://127.0.0.1:1/bot%s/%s")

	_, err := bot.Request(NewMessage(1, "hi"))
	if err == nil {
		t.Fatal("expected a transport error")
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("error leaks the token: %v", err)
	}
}
