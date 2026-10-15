package tgbotapi

import (
	"context"
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

// writeTooManyRequests drains the request body and answers with a Telegram
// 429 Too Many Requests error carrying the given retry_after (in seconds).
func writeTooManyRequests(w http.ResponseWriter, r *http.Request, retryAfter int) {
	_, _ = io.Copy(io.Discard, r.Body)
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = fmt.Fprintf(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after %d","parameters":{"retry_after":%d}}`, retryAfter, retryAfter)
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

func TestMakeRequest_MaxRetryAfterExceeded(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		if call == 1 {
			writeTooManyRequests(w, r, 60)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	})

	bot.MaxRetries = 3
	bot.MaxRetryAfter = 5 * time.Second
	bot.sleepFunc = func(d time.Duration) {
		t.Errorf("sleepFunc should not be called when retry_after exceeds MaxRetryAfter, got %v", d)
	}

	resp, err := bot.MakeRequest("sendMessage", Params{"chat_id": "1"})
	if err == nil {
		t.Fatal("expected 429 error when retry_after exceeds MaxRetryAfter, got nil")
	}

	if retryAfter, ok := IsTooManyRequests(err); !ok || retryAfter != 60*time.Second {
		t.Fatalf("expected (60s, true) from IsTooManyRequests, got (%v, %v)", retryAfter, ok)
	}

	if resp == nil || resp.ErrorCode != http.StatusTooManyRequests {
		t.Fatalf("expected response with error code 429, got %+v", resp)
	}

	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected exactly 1 call when retry_after exceeds MaxRetryAfter, got %d", got)
	}
}

func TestMakeRequest_MaxRetryAfterNotExceeded(t *testing.T) {
	tests := []struct {
		name          string
		retryAfter    int
		maxRetryAfter time.Duration
	}{
		{name: "equal to limit", retryAfter: 5, maxRetryAfter: 5 * time.Second},
		{name: "zero means no limit", retryAfter: 60, maxRetryAfter: 0},
		{name: "negative means no limit", retryAfter: 60, maxRetryAfter: -time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callCount int32
			bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
				call := atomic.AddInt32(&callCount, 1)
				if call == 1 {
					writeTooManyRequests(w, r, tt.retryAfter)
					return
				}
				_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
			})

			bot.MaxRetries = 1
			bot.MaxRetryAfter = tt.maxRetryAfter
			var sleptDurations []time.Duration
			bot.sleepFunc = func(d time.Duration) {
				sleptDurations = append(sleptDurations, d)
			}

			if _, err := bot.MakeRequest("sendMessage", Params{"chat_id": "1"}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := atomic.LoadInt32(&callCount); got != 2 {
				t.Fatalf("expected 2 calls, got %d", got)
			}

			want := time.Duration(tt.retryAfter) * time.Second
			if len(sleptDurations) != 1 || sleptDurations[0] != want {
				t.Fatalf("expected full retry_after sleep [%v], got %v", want, sleptDurations)
			}
		})
	}
}

func TestMakeRequest_NoRetryWithoutRetryAfter(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing parameters",
			body: `{"ok":false,"error_code":429,"description":"Too Many Requests"}`,
		},
		{
			name: "missing retry_after",
			body: `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{}}`,
		},
		{
			name: "zero retry_after",
			body: `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 0","parameters":{"retry_after":0}}`,
		},
		{
			name: "negative retry_after",
			body: `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":-1}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callCount int32
			bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
				call := atomic.AddInt32(&callCount, 1)
				if call == 1 {
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = io.WriteString(w, tt.body)
					return
				}
				_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
			})

			bot.MaxRetries = 3
			bot.sleepFunc = func(d time.Duration) {
				t.Errorf("sleepFunc should not be called without a positive retry_after, got %v", d)
			}

			_, err := bot.MakeRequest("sendMessage", Params{"chat_id": "1"})
			if err == nil {
				t.Fatal("expected 429 error, got nil")
			}

			if retryAfter, ok := IsTooManyRequests(err); !ok || retryAfter > 0 {
				t.Fatalf("expected (<=0, true) from IsTooManyRequests, got (%v, %v)", retryAfter, ok)
			}

			if got := atomic.LoadInt32(&callCount); got != 1 {
				t.Fatalf("expected exactly 1 call without a positive retry_after, got %d", got)
			}
		})
	}
}

func TestSleepDuration_DefaultsToTimeSleep(t *testing.T) {
	bot := &BotAPI{}

	const d = 20 * time.Millisecond
	start := time.Now()
	if err := bot.sleepDuration(context.Background(), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if elapsed := time.Since(start); elapsed < d {
		t.Fatalf("expected sleepDuration to wait at least %v, got %v", d, elapsed)
	}
}

func TestSleepDuration_StopsWhenContextDone(t *testing.T) {
	bot := &BotAPI{}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := bot.sleepDuration(ctx, time.Minute)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("sleepDuration did not stop when the context was done (took %v)", elapsed)
	}
}

func TestMakeRequestWithContext_CancelDuringRetryWait(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		writeTooManyRequests(w, r, 60)
	})
	bot.MaxRetries = 3

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := bot.MakeRequestWithContext(ctx, "sendMessage", Params{"chat_id": "1", "text": "test"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("retry wait ignored the context (took %v)", elapsed)
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
}

func TestUploadFilesWithContext_CancelDuringRetryWait(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		writeTooManyRequests(w, r, 60)
	})
	bot.MaxRetries = 3

	ctx, cancel := context.WithCancel(context.Background())
	bot.sleepFunc = func(time.Duration) { cancel() }

	files := []RequestFile{{Name: "document", Data: FileBytes{Name: "a.txt", Bytes: []byte("payload")}}}
	_, err := bot.UploadFilesWithContext(ctx, "sendDocument", Params{"chat_id": "1"}, files)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
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

// customFileData is a RequestFileData implementation the library knows nothing
// about, so uploads containing it must not be replayed.
type customFileData struct {
	reader io.Reader
}

func (f customFileData) NeedsUpload() bool { return true }

func (f customFileData) UploadData() (string, io.Reader, error) {
	return "custom.bin", f.reader, nil
}

func (f customFileData) SendData() string { panic("customFileData must be uploaded") }

// embeddedFileReader embeds FileReader, so it is a different type and must not
// be mistaken for a replayable upload.
type embeddedFileReader struct {
	FileReader
}

func TestCanReplayUploadFiles(t *testing.T) {
	tests := []struct {
		name  string
		files []RequestFile
		want  bool
	}{
		{name: "no files", files: nil, want: true},
		{name: "FileID", files: []RequestFile{{Name: "photo", Data: FileID("id")}}, want: true},
		{name: "FileURL", files: []RequestFile{{Name: "photo", Data: FileURL("https://example.com/a.jpg")}}, want: true},
		{name: "FileBytes", files: []RequestFile{{Name: "photo", Data: FileBytes{Name: "a.jpg"}}}, want: true},
		{name: "*FileBytes", files: []RequestFile{{Name: "photo", Data: &FileBytes{Name: "a.jpg"}}}, want: true},
		{name: "FilePath", files: []RequestFile{{Name: "photo", Data: FilePath("a.jpg")}}, want: true},
		{name: "*FilePath", files: []RequestFile{{Name: "photo", Data: func() *FilePath { p := FilePath("a.jpg"); return &p }()}}, want: true},
		{name: "FileReader", files: []RequestFile{{Name: "photo", Data: FileReader{Name: "a.jpg"}}}, want: false},
		{name: "*FileReader", files: []RequestFile{{Name: "photo", Data: &FileReader{Name: "a.jpg"}}}, want: false},
		{name: "custom RequestFileData", files: []RequestFile{{Name: "photo", Data: customFileData{}}}, want: false},
		{name: "struct embedding FileReader", files: []RequestFile{{Name: "photo", Data: embeddedFileReader{}}}, want: false},
		{
			name: "FileBytes and FileID",
			files: []RequestFile{
				{Name: "photo", Data: FileBytes{Name: "a.jpg"}},
				{Name: "thumbnail", Data: FileID("id")},
			},
			want: true,
		},
		{
			name: "FileBytes and FileReader",
			files: []RequestFile{
				{Name: "photo", Data: FileBytes{Name: "a.jpg"}},
				{Name: "thumbnail", Data: FileReader{Name: "b.jpg"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canReplayUploadFiles(tt.files); got != tt.want {
				t.Fatalf("canReplayUploadFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUploadFiles_NoRetryForNonReplayableFiles(t *testing.T) {
	tests := []struct {
		name  string
		files []RequestFile
	}{
		{
			name:  "FileReader",
			files: []RequestFile{{Name: "photo", Data: FileReader{Name: "stream.jpg", Reader: strings.NewReader("STREAM_DATA")}}},
		},
		{
			name:  "*FileReader",
			files: []RequestFile{{Name: "photo", Data: &FileReader{Name: "stream.jpg", Reader: strings.NewReader("STREAM_DATA")}}},
		},
		{
			name:  "custom RequestFileData",
			files: []RequestFile{{Name: "document", Data: customFileData{reader: strings.NewReader("CUSTOM_DATA")}}},
		},
		{
			name:  "struct embedding FileReader",
			files: []RequestFile{{Name: "photo", Data: embeddedFileReader{FileReader{Name: "stream.jpg", Reader: strings.NewReader("STREAM_DATA")}}}},
		},
		{
			name: "FileBytes mixed with FileReader",
			files: []RequestFile{
				{Name: "photo", Data: FileBytes{Name: "cat.jpg", Bytes: []byte("CAT_IMAGE_DATA")}},
				{Name: "thumbnail", Data: FileReader{Name: "thumb.jpg", Reader: strings.NewReader("THUMB_DATA")}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callCount int32
			bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
				call := atomic.AddInt32(&callCount, 1)
				if call == 1 {
					writeTooManyRequests(w, r, 1)
					return
				}
				_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":456}}`)
			})

			bot.MaxRetries = 2
			bot.sleepFunc = func(d time.Duration) {
				t.Errorf("sleepFunc should not be called for a non-replayable upload, got %v", d)
			}

			_, err := bot.UploadFiles("sendPhoto", Params{"chat_id": "1"}, tt.files)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if retryAfter, ok := IsTooManyRequests(err); !ok || retryAfter != time.Second {
				t.Fatalf("expected (1s, true) from IsTooManyRequests, got (%v, %v)", retryAfter, ok)
			}

			if got := atomic.LoadInt32(&callCount); got != 1 {
				t.Fatalf("expected exactly 1 call because the upload cannot be replayed, got %d", got)
			}
		})
	}
}

func TestSendMediaGroup_NoRetryWhenMixedWithFileReader(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		if call == 1 {
			writeTooManyRequests(w, r, 1)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":[]}`)
	})

	bot.MaxRetries = 2
	bot.sleepFunc = func(d time.Duration) {
		t.Errorf("sleepFunc should not be called for a media group containing a FileReader, got %v", d)
	}

	cfg := NewMediaGroup(1, []interface{}{
		NewInputMediaPhoto(FileBytes{Name: "a.jpg", Bytes: []byte("A")}),
		NewInputMediaPhoto(FileReader{Name: "b.jpg", Reader: strings.NewReader("B")}),
	})

	_, err := bot.SendMediaGroup(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if retryAfter, ok := IsTooManyRequests(err); !ok || retryAfter != time.Second {
		t.Fatalf("expected (1s, true) from IsTooManyRequests, got (%v, %v)", retryAfter, ok)
	}

	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Fatalf("expected exactly 1 call for a media group containing a FileReader, got %d", got)
	}
}

func TestSendMediaGroup_RetryReplayableFiles(t *testing.T) {
	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		if call == 1 {
			writeTooManyRequests(w, r, 1)
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = io.WriteString(w, `{"ok":true,"result":[]}`)
	})

	bot.MaxRetries = 1
	var sleptDurations []time.Duration
	bot.sleepFunc = func(d time.Duration) {
		sleptDurations = append(sleptDurations, d)
	}

	cfg := NewMediaGroup(1, []interface{}{
		NewInputMediaPhoto(FileBytes{Name: "a.jpg", Bytes: []byte("A")}),
		NewInputMediaPhoto(FileID("existing-file-id")),
	})

	if _, err := bot.SendMediaGroup(cfg); err != nil {
		t.Fatalf("SendMediaGroup: expected success on retry, got: %v", err)
	}

	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}

	if len(sleptDurations) != 1 || sleptDurations[0] != time.Second {
		t.Fatalf("expected 1 sleep for 1s, got %v", sleptDurations)
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

// captureLogger is a BotLogger that records everything logged through it.
type captureLogger struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (l *captureLogger) Println(v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintln(&l.buf, v...)
}

func (l *captureLogger) Printf(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(&l.buf, format, v...)
}

func (l *captureLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func TestMakeRequest_DebugLogsRetries(t *testing.T) {
	logger := &captureLogger{}
	prevLogger := log
	if err := SetLogger(logger); err != nil {
		t.Fatalf("SetLogger: %v", err)
	}
	t.Cleanup(func() { log = prevLogger })

	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		if call <= 2 {
			writeTooManyRequests(w, r, 3)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":123,"text":"test"}}`)
	})

	bot.Debug = true
	bot.MaxRetries = 2
	bot.sleepFunc = func(time.Duration) {}

	if _, err := bot.MakeRequest("sendMessage", Params{"chat_id": "1", "text": "test"}); err != nil {
		t.Fatalf("expected success on retry, got error: %v", err)
	}

	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Fatalf("expected 3 calls, got %d", got)
	}

	output := logger.String()
	for _, want := range []string{
		"Endpoint: sendMessage, 429 Too Many Requests, retrying in 3s (attempt 1/2)",
		"Endpoint: sendMessage, 429 Too Many Requests, retrying in 3s (attempt 2/2)",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected debug log to contain %q, got:\n%s", want, output)
		}
	}
}

func TestMakeRequest_NoRetryLogWithoutDebug(t *testing.T) {
	logger := &captureLogger{}
	prevLogger := log
	if err := SetLogger(logger); err != nil {
		t.Fatalf("SetLogger: %v", err)
	}
	t.Cleanup(func() { log = prevLogger })

	var callCount int32
	bot, _ := newRetryTestBot(t, func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&callCount, 1)
		if call == 1 {
			writeTooManyRequests(w, r, 1)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
	})

	bot.MaxRetries = 1
	bot.sleepFunc = func(time.Duration) {}

	if _, err := bot.MakeRequest("sendMessage", Params{"chat_id": "1"}); err != nil {
		t.Fatalf("expected success on retry, got error: %v", err)
	}

	if output := logger.String(); output != "" {
		t.Fatalf("expected no log output when Debug is false, got:\n%s", output)
	}
}
