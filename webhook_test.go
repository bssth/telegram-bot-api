package tgbotapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// webhookPatternSeq makes every pattern registered on http.DefaultServeMux
// unique, so the tests can run repeatedly (go test -count=N) without a
// "pattern conflicts" panic.
var webhookPatternSeq atomic.Uint64

func uniqueWebhookPattern(name string) string {
	return fmt.Sprintf("/%s-%d", name, webhookPatternSeq.Add(1))
}

// checkWebhookResponse asserts the status code and Content-Type as a client
// receives them. It reads w.Result() rather than w.Code and w.Header(): the
// latter is the live header map, so it would not catch a header set after
// WriteHeader.
func checkWebhookResponse(t *testing.T, w *httptest.ResponseRecorder, wantStatus int, wantContentType string) {
	t.Helper()

	res := w.Result()
	if res.StatusCode != wantStatus {
		t.Fatalf("expected status %d, got %d", wantStatus, res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != wantContentType {
		t.Fatalf("expected Content-Type %q, got %q", wantContentType, ct)
	}
}

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

func TestHandleUpdate_SecretRequired(t *testing.T) {
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

	// The secret is checked before the HTTP method and the body.
	reqGet := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	if _, err = bot.HandleUpdate(reqGet); !errors.Is(err, ErrBadSecretToken) {
		t.Fatalf("expected ErrBadSecretToken for GET without secret, got: %v", err)
	}
	reqBadJSON := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`not json`))
	if _, err = bot.HandleUpdate(reqBadJSON); !errors.Is(err, ErrBadSecretToken) {
		t.Fatalf("expected ErrBadSecretToken for invalid JSON without secret, got: %v", err)
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

		if code := w.Result().StatusCode; code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", code)
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

		checkWebhookResponse(t, w, http.StatusUnauthorized, "application/json")

		var resp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response json: %v", err)
		}
		if resp["error"] != ErrBadSecretToken.Error() {
			t.Fatalf("expected error message %q, got %q", ErrBadSecretToken.Error(), resp["error"])
		}
	})

	t.Run("missing secret returns 401 and sets Content-Type header", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id": 403}`))

		ch := bot.ListenForWebhookRespReqFormat(w, r)
		if u, ok := <-ch; ok {
			t.Fatalf("expected no update, received: %+v", u)
		}

		checkWebhookResponse(t, w, http.StatusUnauthorized, "application/json")
	})

	t.Run("bad json payload returns 400 and sets Content-Type header", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`bad json`))
		r.Header.Set(WebhookSecretTokenHeader, secret)

		ch := bot.ListenForWebhookRespReqFormat(w, r)
		if u, ok := <-ch; ok {
			t.Fatalf("expected no update, received: %+v", u)
		}

		checkWebhookResponse(t, w, http.StatusBadRequest, "application/json")
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

		if code := w.Result().StatusCode; code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", code)
		}
	})
}

func TestListenForWebhook(t *testing.T) {
	secret := "listen-webhook-secret"
	bot := &BotAPI{
		Buffer:             10,
		WebhookSecretToken: secret,
	}

	pattern := uniqueWebhookPattern("test-webhook-listen")
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

	checkWebhookResponse(t, wWrong, http.StatusUnauthorized, "application/json")

	// Invalid request without secret
	reqMissing := httptest.NewRequest(http.MethodPost, pattern, strings.NewReader(`{"update_id": 503}`))
	wMissing := httptest.NewRecorder()
	http.DefaultServeMux.ServeHTTP(wMissing, reqMissing)

	checkWebhookResponse(t, wMissing, http.StatusUnauthorized, "application/json")

	select {
	case u := <-ch:
		t.Fatalf("expected no update for rejected requests, received: %+v", u)
	default:
	}
}

func TestListenForWebhook_WithoutSecret(t *testing.T) {
	bot := &BotAPI{Buffer: 10}
	pattern := uniqueWebhookPattern("test-webhook-no-secret")
	ch := bot.ListenForWebhook(pattern)

	reqValid := httptest.NewRequest(http.MethodPost, pattern, strings.NewReader(`{"update_id": 601}`))
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

	reqBadJSON := httptest.NewRequest(http.MethodPost, pattern, strings.NewReader(`bad json`))
	wBadJSON := httptest.NewRecorder()
	http.DefaultServeMux.ServeHTTP(wBadJSON, reqBadJSON)

	checkWebhookResponse(t, wBadJSON, http.StatusBadRequest, "application/json")
}

func TestListenForWebhook_SecretSetAfterRegistration(t *testing.T) {
	bot := &BotAPI{Buffer: 10}
	pattern := uniqueWebhookPattern("test-webhook-secret-after")
	ch := bot.ListenForWebhook(pattern)

	// The secret is configured only after the handler has been registered;
	// the handler must still enforce it.
	secret := "late-secret-value"
	bot.WebhookSecretToken = secret

	reqMissing := httptest.NewRequest(http.MethodPost, pattern, strings.NewReader(`{"update_id": 701}`))
	wMissing := httptest.NewRecorder()
	http.DefaultServeMux.ServeHTTP(wMissing, reqMissing)

	checkWebhookResponse(t, wMissing, http.StatusUnauthorized, "application/json")

	select {
	case u := <-ch:
		t.Fatalf("expected no update without secret header, received: %+v", u)
	default:
	}

	reqValid := httptest.NewRequest(http.MethodPost, pattern, strings.NewReader(`{"update_id": 702}`))
	reqValid.Header.Set(WebhookSecretTokenHeader, secret)
	wValid := httptest.NewRecorder()
	http.DefaultServeMux.ServeHTTP(wValid, reqValid)

	select {
	case u := <-ch:
		if u.UpdateID != 702 {
			t.Fatalf("expected update_id 702, got %d", u.UpdateID)
		}
	default:
		t.Fatal("expected update in channel")
	}
}
