package tgbotapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsTooManyRequests(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		duration, ok := IsTooManyRequests(nil)
		if ok || duration != 0 {
			t.Fatalf("expected (0, false), got (%v, %v)", duration, ok)
		}
	})

	t.Run("generic error", func(t *testing.T) {
		duration, ok := IsTooManyRequests(errors.New("some network error"))
		if ok || duration != 0 {
			t.Fatalf("expected (0, false), got (%v, %v)", duration, ok)
		}
	})

	t.Run("Error pointer with 400", func(t *testing.T) {
		err := &Error{
			Code:    http.StatusBadRequest,
			Message: "Bad Request: chat not found",
		}
		duration, ok := IsTooManyRequests(err)
		if ok || duration != 0 {
			t.Fatalf("expected (0, false), got (%v, %v)", duration, ok)
		}
	})

	t.Run("Error pointer with 429", func(t *testing.T) {
		err := &Error{
			Code:    http.StatusTooManyRequests,
			Message: "Too Many Requests: retry after 5",
			ResponseParameters: ResponseParameters{
				RetryAfter: 5,
			},
		}
		duration, ok := IsTooManyRequests(err)
		if !ok || duration != 5*time.Second {
			t.Fatalf("expected (5s, true), got (%v, %v)", duration, ok)
		}
	})

	t.Run("Error value with 429", func(t *testing.T) {
		err := Error{
			Code:    http.StatusTooManyRequests,
			Message: "Too Many Requests: retry after 3",
			ResponseParameters: ResponseParameters{
				RetryAfter: 3,
			},
		}
		duration, ok := IsTooManyRequests(err)
		if !ok || duration != 3*time.Second {
			t.Fatalf("expected (3s, true), got (%v, %v)", duration, ok)
		}
	})

	t.Run("wrapped Error with 429", func(t *testing.T) {
		err := fmt.Errorf("request failed: %w", &Error{
			Code:    http.StatusTooManyRequests,
			Message: "Too Many Requests: retry after 10",
			ResponseParameters: ResponseParameters{
				RetryAfter: 10,
			},
		})
		duration, ok := IsTooManyRequests(err)
		if !ok || duration != 10*time.Second {
			t.Fatalf("expected (10s, true), got (%v, %v)", duration, ok)
		}
	})

	t.Run("429 with zero RetryAfter", func(t *testing.T) {
		err := &Error{
			Code:    http.StatusTooManyRequests,
			Message: "Too Many Requests",
		}
		duration, ok := IsTooManyRequests(err)
		if !ok || duration != 0 {
			t.Fatalf("expected (0, true), got (%v, %v)", duration, ok)
		}
	})
}

func newRetryTestBot(t *testing.T, handler http.HandlerFunc) (*BotAPI, *httptest.Server) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getMe") {
			_, _ = io.WriteString(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"Bot","username":"bot"}}`)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	bot, err := NewBotAPIWithAPIEndpoint("TOKEN", srv.URL+"/bot%s/%s")
	if err != nil {
		t.Fatalf("NewBotAPIWithAPIEndpoint: %v", err)
	}

	return bot, srv
}

func TestMakeRequest_RetryDisabledByDefault(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}`)
	})

	if bot.MaxRetries != 0 {
		t.Fatalf("expected MaxRetries to default to 0, got %d", bot.MaxRetries)
	}

	_, err := bot.MakeRequest("sendMessage", Params{"chat_id": "1", "text": "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if retryAfter, ok := IsTooManyRequests(err); !ok || retryAfter != time.Second {
		t.Fatalf("expected (1s, true) from IsTooManyRequests, got (%v, %v)", retryAfter, ok)
	}

	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected exactly 1 call when retry is disabled, got %d", got)
	}
}

func TestMakeRequest_RetrySucceeds(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		if call == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":123,"text":"test"}}`)
	})

	bot.MaxRetries = 1
	var sleptDurations []time.Duration
	bot.sleepFunc = func(d time.Duration) {
		sleptDurations = append(sleptDurations, d)
	}

	resp, err := bot.MakeRequest("sendMessage", Params{"chat_id": "1", "text": "test"})
	if err != nil {
		t.Fatalf("expected success on retry, got error: %v", err)
	}

	if !resp.Ok {
		t.Fatal("expected response Ok to be true")
	}

	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}

	if len(sleptDurations) != 1 || sleptDurations[0] != time.Second {
		t.Fatalf("expected sleptDurations [1s], got %v", sleptDurations)
	}
}

func TestMakeRequest_RetryCountRespected(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 2","parameters":{"retry_after":2}}`)
	})

	bot.MaxRetries = 2
	var sleptDurations []time.Duration
	bot.sleepFunc = func(d time.Duration) {
		sleptDurations = append(sleptDurations, d)
	}

	_, err := bot.MakeRequest("sendMessage", Params{"chat_id": "1", "text": "test"})
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}

	if retryAfter, ok := IsTooManyRequests(err); !ok || retryAfter != 2*time.Second {
		t.Fatalf("expected (2s, true) from IsTooManyRequests, got (%v, %v)", retryAfter, ok)
	}

	// 1 initial request + 2 retries = 3 total requests
	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Fatalf("expected 3 calls for MaxRetries=2, got %d", got)
	}

	if len(sleptDurations) != 2 {
		t.Fatalf("expected 2 sleeps, got %d (%v)", len(sleptDurations), sleptDurations)
	}
}

func TestMakeRequest_MaxRetryAfterCapping(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		if call == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 60","parameters":{"retry_after":60}}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	})

	bot.MaxRetries = 1
	bot.MaxRetryAfter = 5 * time.Second
	var sleptDurations []time.Duration
	bot.sleepFunc = func(d time.Duration) {
		sleptDurations = append(sleptDurations, d)
	}

	_, err := bot.MakeRequest("sendMessage", Params{"chat_id": "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sleptDurations) != 1 || sleptDurations[0] != 5*time.Second {
		t.Fatalf("expected sleep capped to 5s, got %v", sleptDurations)
	}
}

func TestMakeRequest_RealSleep(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		if call == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 10","parameters":{"retry_after":10}}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	})

	bot.MaxRetries = 1
	// Cap wait to 20ms so test runs fast with actual time.Sleep.
	bot.MaxRetryAfter = 20 * time.Millisecond

	start := time.Now()
	_, err := bot.MakeRequest("sendMessage", Params{"chat_id": "1"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed < 15*time.Millisecond {
		t.Fatalf("expected elapsed time >= 15ms, got %v", elapsed)
	}
	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
}

func TestMakeRequest_Non429ErrorNotRetried(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"ok":false,"error_code":400,"description":"Bad Request: message text is empty"}`)
	})

	bot.MaxRetries = 3
	_, err := bot.MakeRequest("sendMessage", Params{"chat_id": "1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected 1 call for 400 error, got %d", got)
	}
}

func TestUploadFiles_RetryReplayable(t *testing.T) {
	var mu sync.Mutex
	var callCount int
	var receivedFiles []string

	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		call := callCount
		mu.Unlock()

		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
		}
		if fHeaders, ok := r.MultipartForm.File["photo"]; ok && len(fHeaders) > 0 {
			f, _ := fHeaders[0].Open()
			b, _ := io.ReadAll(f)
			_ = f.Close()
			mu.Lock()
			receivedFiles = append(receivedFiles, string(b))
			mu.Unlock()
		}

		if call == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}`)
			return
		}

		_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":456}}`)
	})

	bot.MaxRetries = 1
	var sleptDurations []time.Duration
	bot.sleepFunc = func(d time.Duration) {
		sleptDurations = append(sleptDurations, d)
	}

	files := []RequestFile{
		{Name: "photo", Data: FileBytes{Name: "cat.jpg", Bytes: []byte("CAT_IMAGE_DATA")}},
	}

	resp, err := bot.UploadFiles("sendPhoto", Params{"chat_id": "1"}, files)
	if err != nil {
		t.Fatalf("UploadFiles: expected success on retry, got: %v", err)
	}
	if !resp.Ok {
		t.Fatal("expected response Ok to be true")
	}

	mu.Lock()
	defer mu.Unlock()
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}
	if len(receivedFiles) != 2 || receivedFiles[1] != "CAT_IMAGE_DATA" {
		t.Fatalf("expected replayed file data 'CAT_IMAGE_DATA', got %v", receivedFiles)
	}
	if len(sleptDurations) != 1 || sleptDurations[0] != time.Second {
		t.Fatalf("expected 1 sleep for 1s, got %v", sleptDurations)
	}
}

func TestUploadFiles_RetryFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(filePath, []byte("FILE_PATH_DATA"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var mu sync.Mutex
	var callCount int
	var receivedFiles []string

	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		call := callCount
		mu.Unlock()

		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
		}
		if fHeaders, ok := r.MultipartForm.File["document"]; ok && len(fHeaders) > 0 {
			f, _ := fHeaders[0].Open()
			b, _ := io.ReadAll(f)
			_ = f.Close()
			mu.Lock()
			receivedFiles = append(receivedFiles, string(b))
			mu.Unlock()
		}

		if call == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}`)
			return
		}

		_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":789}}`)
	})

	bot.MaxRetries = 1
	bot.sleepFunc = func(d time.Duration) {}

	files := []RequestFile{
		{Name: "document", Data: FilePath(filePath)},
	}

	resp, err := bot.UploadFiles("sendDocument", Params{"chat_id": "1"}, files)
	if err != nil {
		t.Fatalf("UploadFiles: expected success on retry, got: %v", err)
	}
	if !resp.Ok {
		t.Fatal("expected response Ok to be true")
	}

	mu.Lock()
	defer mu.Unlock()
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}
	if len(receivedFiles) != 2 || receivedFiles[1] != "FILE_PATH_DATA" {
		t.Fatalf("expected replayed file data 'FILE_PATH_DATA', got %v", receivedFiles)
	}
}

func TestUploadFiles_NoRetryForFileReader(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}`)
	})

	bot.MaxRetries = 2
	bot.sleepFunc = func(d time.Duration) {
		t.Error("sleepFunc should not be called when FileReader upload fails")
	}

	reader := strings.NewReader("STREAM_DATA")
	files := []RequestFile{
		{Name: "photo", Data: FileReader{Name: "stream.jpg", Reader: reader}},
	}

	_, err := bot.UploadFiles("sendPhoto", Params{"chat_id": "1"}, files)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if retryAfter, ok := IsTooManyRequests(err); !ok || retryAfter != time.Second {
		t.Fatalf("expected (1s, true) from IsTooManyRequests, got (%v, %v)", retryAfter, ok)
	}

	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected exactly 1 call because FileReader cannot be replayed, got %d", got)
	}
}

func TestSend_RetryWithChattable(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		if call == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 2","parameters":{"retry_after":2}}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":999,"text":"hello"}}`)
	})

	bot.MaxRetries = 1
	bot.sleepFunc = func(d time.Duration) {}

	msg, err := bot.Send(NewMessage(1, "hello"))
	if err != nil {
		t.Fatalf("Send: unexpected error: %v", err)
	}
	if msg.MessageID != 999 {
		t.Fatalf("expected message ID 999, got %d", msg.MessageID)
	}
	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
}
