package tgbotapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetChatMemberCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/getMe") {
			io.WriteString(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Bot"}}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/getChatMemberCount") {
			io.WriteString(w, `{"ok":true,"result":42}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	bot, err := NewBotAPIWithAPIEndpoint("TOKEN", srv.URL+"/bot%s/%s")
	if err != nil {
		t.Fatal(err)
	}

	count, err := bot.GetChatMemberCount(ChatMemberCountConfig{ChatConfig: ChatConfig{ChatID: 12345}})
	if err != nil {
		t.Fatalf("GetChatMemberCount failed: %v", err)
	}
	if count != 42 {
		t.Fatalf("expected count 42, got %d", count)
	}
}

func TestBanChatSenderChatConfigParams(t *testing.T) {
	cfg := BanChatSenderChatConfig{
		ChatID:       100,
		SenderChatID: 200,
	}
	params, err := cfg.params()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params["chat_id"] != "100" {
		t.Errorf("expected chat_id 100, got %s", params["chat_id"])
	}
	if params["sender_chat_id"] != "200" {
		t.Errorf("expected sender_chat_id 200, got %s", params["sender_chat_id"])
	}
	if _, ok := params["until_date"]; ok {
		t.Errorf("until_date should not be present in BanChatSenderChatConfig params")
	}
}

func TestInlineQueryResultCachedStickerAndPhotoCleanups(t *testing.T) {
	sticker := NewInlineQueryResultCachedSticker("s1", "sticker123")
	if sticker.Type != "sticker" || sticker.ID != "s1" || sticker.StickerID != "sticker123" {
		t.Fatalf("unexpected sticker result: %+v", sticker)
	}
	data, err := json.Marshal(sticker)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "title") {
		t.Errorf("InlineQueryResultCachedSticker should not contain title in JSON: %s", data)
	}

	photo := InlineQueryResultPhoto{
		Type: "photo",
		ID:   "p1",
		URL:  "https://example.com/photo.jpg",
	}
	photoData, err := json.Marshal(photo)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(photoData), "mime_type") {
		t.Errorf("InlineQueryResultPhoto should not contain mime_type in JSON: %s", photoData)
	}
}

func TestEphemeralMessageParametersEmbedding(t *testing.T) {
	msg := NewMessage(12345, "hello")
	msg.ReceiverUserID = 999
	msg.CallbackQueryID = "cb123"
	msg.ReplaceCallbackQueryMessage = true

	params, err := msg.params()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"receiver_user_id":999,"callback_query_id":"cb123","replace_callback_query_message":true}`
	if params["ephemeral_message_parameters"] != want {
		t.Errorf("got %q, want %q", params["ephemeral_message_parameters"], want)
	}
}

func TestParamsAddAnyUsage(t *testing.T) {
	p := make(Params)
	var nilPtr *string
	if err := p.AddAny("nil", nilPtr); err != nil {
		t.Fatal(err)
	}
	if _, ok := p["nil"]; ok {
		t.Errorf("expected nilPtr to be ignored")
	}

	val := struct {
		Foo string `json:"foo"`
	}{Foo: "bar"}
	if err := p.AddAny("obj", val); err != nil {
		t.Fatal(err)
	}
	if p["obj"] != `{"foo":"bar"}` {
		t.Errorf("expected JSON object, got %s", p["obj"])
	}
}
