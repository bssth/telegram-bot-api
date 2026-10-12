package tgbotapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestBotAPI(serverURL string) *BotAPI {
	bot := &BotAPI{
		Token:  "test-token",
		Client: &http.Client{Timeout: 30 * time.Second},
		Buffer: 100,
	}
	bot.SetAPIEndpoint(serverURL + "/bot%s/%s")
	bot.SetFileEndpoint(serverURL + "/file/bot%s/%s")
	return bot
}

func TestMakeRequestWithContext_Cancelled(t *testing.T) {
	reqReceived := make(chan struct{})
	stopServer := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(reqReceived)
		select {
		case <-r.Context().Done():
		case <-stopServer:
		}
	}))
	defer srv.Close()
	defer close(stopServer)

	bot := newTestBotAPI(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		_, err := bot.MakeRequestWithContext(ctx, "getMe", Params{})
		errCh <- err
	}()

	select {
	case <-reqReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("server never received request")
	}

	cancel()

	select {
	case err := <-errCh:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("MakeRequestWithContext did not return promptly after cancel")
	}
}

func TestUploadFilesWithContext_Cancelled(t *testing.T) {
	reqReceived := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(reqReceived)
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	defer srv.Close()

	bot := newTestBotAPI(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		file := FileBytes{Name: "test.txt", Bytes: []byte("test content")}
		reqFile := RequestFile{Name: "document", Data: file}
		_, err := bot.UploadFilesWithContext(ctx, "sendDocument", Params{"chat_id": "123"}, []RequestFile{reqFile})
		errCh <- err
	}()

	select {
	case <-reqReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("server never received upload request")
	}

	cancel()

	select {
	case err := <-errCh:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("UploadFilesWithContext did not return promptly after cancel")
	}
}

func TestRequestWithContext_Cancelled(t *testing.T) {
	reqReceived := make(chan struct{})
	stopServer := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(reqReceived)
		select {
		case <-r.Context().Done():
		case <-stopServer:
		}
	}))
	defer srv.Close()
	defer close(stopServer)

	bot := newTestBotAPI(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		msg := NewMessage(123, "test")
		_, err := bot.RequestWithContext(ctx, msg)
		errCh <- err
	}()

	select {
	case <-reqReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("server never received request")
	}

	cancel()

	select {
	case err := <-errCh:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RequestWithContext did not return promptly after cancel")
	}
}

func TestSendWithContext_Cancelled(t *testing.T) {
	reqReceived := make(chan struct{})
	stopServer := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(reqReceived)
		select {
		case <-r.Context().Done():
		case <-stopServer:
		}
	}))
	defer srv.Close()
	defer close(stopServer)

	bot := newTestBotAPI(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		msg := NewMessage(123, "test")
		_, err := bot.SendWithContext(ctx, msg)
		errCh <- err
	}()

	select {
	case <-reqReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("server never received request")
	}

	cancel()

	select {
	case err := <-errCh:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SendWithContext did not return promptly after cancel")
	}
}

func TestGetUpdatesChanWithContext_Cancellation(t *testing.T) {
	pollStarted := make(chan struct{}, 1)
	stopServer := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "getUpdates") {
			select {
			case pollStarted <- struct{}{}:
			default:
			}
			select {
			case <-r.Context().Done():
			case <-stopServer:
			}
		}
	}))
	defer srv.Close()
	defer close(stopServer)

	bot := newTestBotAPI(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	uCfg := NewUpdate(0)
	uCfg.Timeout = 60

	updates := bot.GetUpdatesChanWithContext(ctx, uCfg)

	select {
	case <-pollStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("long poll request never started")
	}

	cancel()

	select {
	case _, ok := <-updates:
		if ok {
			t.Fatal("expected updates channel to be closed, but received value")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("updates channel did not close promptly upon context cancellation")
	}
}

func TestGetUpdatesChanWithContext_RetrySleepInterrupted(t *testing.T) {
	firstRequest := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "getUpdates") {
			select {
			case firstRequest <- struct{}{}:
			default:
			}
			// Respond with an internal server error so bot sleeps for 3 seconds before retrying
			http.Error(w, `{"ok":false,"error_code":500,"description":"internal error"}`, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	bot := newTestBotAPI(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	uCfg := NewUpdate(0)

	start := time.Now()
	updates := bot.GetUpdatesChanWithContext(ctx, uCfg)

	select {
	case <-firstRequest:
	case <-time.After(2 * time.Second):
		t.Fatal("first getUpdates request never received")
	}

	// Wait 50ms so getUpdates fails and enters the 3s sleep
	time.Sleep(50 * time.Millisecond)

	cancel()

	select {
	case _, ok := <-updates:
		if ok {
			t.Fatal("expected updates channel to be closed, but received value")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("updates channel did not close promptly during retry sleep")
	}

	if elapsed := time.Since(start); elapsed >= 3*time.Second {
		t.Fatalf("channel closing took %v, retry sleep was not interrupted", elapsed)
	}
}

func TestStopReceivingUpdates_Idempotent(t *testing.T) {
	bot := &BotAPI{
		Token:  "test-token",
		Client: &http.Client{},
	}

	// Calling StopReceivingUpdates before any GetUpdatesChan should not panic
	bot.StopReceivingUpdates()
	bot.StopReceivingUpdates()

	stopServer := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-stopServer:
		}
	}))
	defer srv.Close()
	defer close(stopServer)

	bot = newTestBotAPI(srv.URL)
	uCfg := NewUpdate(0)
	updates := bot.GetUpdatesChan(uCfg)

	// Calling StopReceivingUpdates multiple times should not panic
	bot.StopReceivingUpdates()
	bot.StopReceivingUpdates()
	bot.StopReceivingUpdates()

	select {
	case _, ok := <-updates:
		if ok {
			t.Fatal("expected updates channel to be closed")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("updates channel did not close promptly after StopReceivingUpdates")
	}
}

func TestGetUpdatesChan_PromptStopOnInFlightPoll(t *testing.T) {
	pollStarted := make(chan struct{}, 1)
	stopServer := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "getUpdates") {
			select {
			case pollStarted <- struct{}{}:
			default:
			}
			select {
			case <-r.Context().Done():
			case <-stopServer:
			}
		}
	}))
	defer srv.Close()
	defer close(stopServer)

	bot := newTestBotAPI(srv.URL)
	uCfg := NewUpdate(0)
	uCfg.Timeout = 60

	updates := bot.GetUpdatesChan(uCfg)

	select {
	case <-pollStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("long poll request never started")
	}

	bot.StopReceivingUpdates()

	select {
	case _, ok := <-updates:
		if ok {
			t.Fatal("expected updates channel to be closed, but received value")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("updates channel did not close promptly after StopReceivingUpdates")
	}
}

func TestGetUpdatesChan_Restartable(t *testing.T) {
	var mu sync.Mutex
	updateCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "getUpdates") {
			mu.Lock()
			count := updateCount
			mu.Unlock()
			resp := fmt.Sprintf(`{"ok":true,"result":[{"update_id":%d}]}`, count+1)
			io.WriteString(w, resp)
		}
	}))
	defer srv.Close()

	bot := newTestBotAPI(srv.URL)
	uCfg := NewUpdate(0)

	// First run
	updates1 := bot.GetUpdatesChan(uCfg)
	select {
	case upd, ok := <-updates1:
		if !ok || upd.UpdateID != 1 {
			t.Fatalf("expected update 1, got %v, ok=%v", upd, ok)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for update on first run")
	}

	bot.StopReceivingUpdates()

	select {
	case _, ok := <-updates1:
		if ok {
			t.Fatal("expected updates1 to be closed")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("updates1 did not close")
	}

	// Restart on the same BotAPI instance
	mu.Lock()
	updateCount = 10
	mu.Unlock()

	updates2 := bot.GetUpdatesChan(uCfg)
	select {
	case upd, ok := <-updates2:
		if !ok || upd.UpdateID != 11 {
			t.Fatalf("expected update 11 on restart, got %v, ok=%v", upd, ok)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for update on restarted run")
	}

	bot.StopReceivingUpdates()

	select {
	case _, ok := <-updates2:
		if ok {
			t.Fatal("expected updates2 to be closed")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("updates2 did not close")
	}
}

func TestTypedHelpersWithContext_Cancelled(t *testing.T) {
	reqReceived := make(chan struct{})
	stopServer := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(reqReceived)
		select {
		case <-r.Context().Done():
		case <-stopServer:
		}
	}))
	defer srv.Close()
	defer close(stopServer)

	bot := newTestBotAPI(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		_, err := bot.GetMeWithContext(ctx)
		errCh <- err
	}()

	select {
	case <-reqReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("server never received GetMe request")
	}

	cancel()

	select {
	case err := <-errCh:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetMeWithContext did not return promptly after cancel")
	}
}
