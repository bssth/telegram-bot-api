package tgbotapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleUpdate_WithoutSecret(t *testing.T) {
	bot := &BotAPI{}

	// Request without secret header should succeed when no secret is configured.
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 100}`))
	update, err := bot.HandleUpdate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update == nil || update.UpdateID != 100 {
		t.Fatalf("unexpected update: %+v", update)
	}

	// Request with an arbitrary secret header should also succeed when no secret is configured.
	reqWithHeader := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 101}`))
	reqWithHeader.Header.Set(WebhookSecretTokenHeader, "some-secret")
	update, err = bot.HandleUpdate(reqWithHeader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update == nil || update.UpdateID != 101 {
		t.Fatalf("unexpected update: %+v", update)
	}

	// Non-POST request should fail.
	reqGet := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	_, err = bot.HandleUpdate(reqGet)
	if err == nil {
		t.Fatal("expected error for GET request, got nil")
	}

	// Invalid JSON should fail.
	reqBadJSON := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`not json`))
	_, err = bot.HandleUpdate(reqBadJSON)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestHandleUpdate_WithSecret(t *testing.T) {
	secret := "test-secret-token-12345"
	bot := &BotAPI{
		WebhookSecretToken: secret,
	}

	// Correct secret header: accepted.
	reqValid := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 200}`))
	reqValid.Header.Set(WebhookSecretTokenHeader, secret)
	update, err := bot.HandleUpdate(reqValid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update == nil || update.UpdateID != 200 {
		t.Fatalf("unexpected update: %+v", update)
	}

	// Wrong secret header: rejected.
	reqWrong := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 201}`))
	reqWrong.Header.Set(WebhookSecretTokenHeader, "wrong-token")
	_, err = bot.HandleUpdate(reqWrong)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
	if !errors.Is(err, ErrBadSecretToken) {
		t.Fatalf("expected ErrBadSecretToken, got: %v", err)
	}

	// Missing secret header: rejected.
	reqMissing := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 202}`))
	_, err = bot.HandleUpdate(reqMissing)
	if err == nil {
		t.Fatal("expected error for missing secret header, got nil")
	}
	if !errors.Is(err, ErrBadSecretToken) {
		t.Fatalf("expected ErrBadSecretToken, got: %v", err)
	}
}

func TestHandleUpdateWithSecret(t *testing.T) {
	bot := &BotAPI{}
	secret := "custom-secret-999"

	// Correct secret: accepted.
	reqValid := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 300}`))
	reqValid.Header.Set(WebhookSecretTokenHeader, secret)
	update, err := bot.HandleUpdateWithSecret(reqValid, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update == nil || update.UpdateID != 300 {
		t.Fatalf("unexpected update: %+v", update)
	}

	// Wrong secret: rejected.
	reqWrong := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 301}`))
	reqWrong.Header.Set(WebhookSecretTokenHeader, "wrong")
	_, err = bot.HandleUpdateWithSecret(reqWrong, secret)
	if !errors.Is(err, ErrBadSecretToken) {
		t.Fatalf("expected ErrBadSecretToken, got: %v", err)
	}

	// Missing secret: rejected.
	reqMissing := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 302}`))
	_, err = bot.HandleUpdateWithSecret(reqMissing, secret)
	if !errors.Is(err, ErrBadSecretToken) {
		t.Fatalf("expected ErrBadSecretToken, got: %v", err)
	}

	// Empty secret: accepted without header check.
	reqEmptySecret := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 303}`))
	update, err = bot.HandleUpdateWithSecret(reqEmptySecret, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update == nil || update.UpdateID != 303 {
		t.Fatalf("unexpected update: %+v", update)
	}
}

func TestListenForWebhookRespReqFormat(t *testing.T) {
	secret := "resp-req-secret"
	bot := &BotAPI{
		Buffer:             10,
		WebhookSecretToken: secret,
	}

	t.Run("correct secret accepted", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 401}`))
		r.Header.Set(WebhookSecretTokenHeader, secret)

		ch := bot.ListenForWebhookRespReqFormat(w, r)
		select {
		case u, ok := <-ch:
			if !ok {
				t.Fatal("channel closed unexpectedly")
			}
			if u.UpdateID != 401 {
				t.Fatalf("expected update_id 401, got %d", u.UpdateID)
			}
		default:
			t.Fatal("expected update in channel")
		}

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("wrong secret returns 401 and sets Content-Type header", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 402}`))
		r.Header.Set(WebhookSecretTokenHeader, "wrong")

		ch := bot.ListenForWebhookRespReqFormat(w, r)
		if u, ok := <-ch; ok {
			t.Fatalf("expected no update, received: %+v", u)
		}

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401 Unauthorized, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected Content-Type application/json, got %q", ct)
		}

		var resp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response json: %v", err)
		}
		if resp["error"] != "bad secret token" {
			t.Fatalf("expected error message %q, got %q", "bad secret token", resp["error"])
		}
	})

	t.Run("missing secret returns 401 and sets Content-Type header", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 403}`))

		ch := bot.ListenForWebhookRespReqFormat(w, r)
		if u, ok := <-ch; ok {
			t.Fatalf("expected no update, received: %+v", u)
		}

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401 Unauthorized, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected Content-Type application/json, got %q", ct)
		}
	})

	t.Run("bad json payload returns 400 and sets Content-Type header", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`bad json`))
		r.Header.Set(WebhookSecretTokenHeader, secret)

		ch := bot.ListenForWebhookRespReqFormat(w, r)
		if u, ok := <-ch; ok {
			t.Fatalf("expected no update, received: %+v", u)
		}

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 Bad Request, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected Content-Type application/json, got %q", ct)
		}
	})

	t.Run("unconfigured secret accepts request without header", func(t *testing.T) {
		noSecretBot := &BotAPI{Buffer: 10}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 404}`))

		ch := noSecretBot.ListenForWebhookRespReqFormat(w, r)
		select {
		case u, ok := <-ch:
			if !ok {
				t.Fatal("channel closed unexpectedly")
			}
			if u.UpdateID != 404 {
				t.Fatalf("expected update_id 404, got %d", u.UpdateID)
			}
		default:
			t.Fatal("expected update in channel")
		}

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
	})
}

func TestListenForWebhook(t *testing.T) {
	secret := "listen-webhook-secret"
	bot := &BotAPI{
		Buffer:             10,
		WebhookSecretToken: secret,
	}

	pattern := "/test-webhook-listen"
	ch := bot.ListenForWebhook(pattern)

	// Valid request with secret
	reqValid := httptest.NewRequest(http.MethodPost, pattern, strings.NewReader(`{"update_id": 501}`))
	reqValid.Header.Set(WebhookSecretTokenHeader, secret)
	wValid := httptest.NewRecorder()
	http.DefaultServeMux.ServeHTTP(wValid, reqValid)

	select {
	case u := <-ch:
		if u.UpdateID != 501 {
			t.Fatalf("expected update_id 501, got %d", u.UpdateID)
		}
	default:
		t.Fatal("expected update in channel")
	}

	// Invalid request with wrong secret
	reqWrong := httptest.NewRequest(http.MethodPost, pattern, strings.NewReader(`{"update_id": 502}`))
	reqWrong.Header.Set(WebhookSecretTokenHeader, "wrong-secret")
	wWrong := httptest.NewRecorder()
	http.DefaultServeMux.ServeHTTP(wWrong, reqWrong)

	if wWrong.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", wWrong.Code)
	}
	if ct := wWrong.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}
}

func TestListenForWebhookWithSecret(t *testing.T) {
	bot := &BotAPI{Buffer: 10}
	pattern := "/test-webhook-with-secret"
	secret := "explicit-secret-value"
	ch := bot.ListenForWebhookWithSecret(pattern, secret)

	reqValid := httptest.NewRequest(http.MethodPost, pattern, strings.NewReader(`{"update_id": 601}`))
	reqValid.Header.Set(WebhookSecretTokenHeader, secret)
	wValid := httptest.NewRecorder()
	http.DefaultServeMux.ServeHTTP(wValid, reqValid)

	select {
	case u := <-ch:
		if u.UpdateID != 601 {
			t.Fatalf("expected update_id 601, got %d", u.UpdateID)
		}
	default:
		t.Fatal("expected update in channel")
	}

	reqMissing := httptest.NewRequest(http.MethodPost, pattern, strings.NewReader(`{"update_id": 602}`))
	wMissing := httptest.NewRecorder()
	http.DefaultServeMux.ServeHTTP(wMissing, reqMissing)

	if wMissing.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", wMissing.Code)
	}
	if ct := wMissing.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}
}
