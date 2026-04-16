package tgbotapi

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

type mockQueueExecutor struct {
	mu           sync.Mutex
	calls        int
	makeReqCalls int
	onCall       func(int) (*APIResponse, error)
}

func (m *mockQueueExecutor) SetDebug(debug bool) {}

func (m *mockQueueExecutor) Debug() bool {
	return false
}

func (m *mockQueueExecutor) SetApiEndpoint(apiEndpoint string) {}

func (m *mockQueueExecutor) MakeRequest(endpoint string, params Params) (*APIResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.makeReqCalls++
	return m.onCall(m.makeReqCalls - 1)
}

func (m *mockQueueExecutor) Request(c Chattable) (*APIResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.onCall(m.calls - 1)
}

func (m *mockQueueExecutor) UploadFiles(endpoint string, params Params, files []RequestFile) (*APIResponse, error) {
	return nil, nil
}

func TestQueuedExecutor_RateLimit(t *testing.T) {
	mock := &mockQueueExecutor{
		onCall: func(n int) (*APIResponse, error) {
			if n == 0 {
				return &APIResponse{
					Ok:        false,
					ErrorCode: 429,
					Parameters: &ResponseParameters{
						RetryAfter: 1,
					},
				}, &Error{Code: 429, Message: "Too Many Requests"}
			}
			return &APIResponse{Ok: true, Result: json.RawMessage(`{"message_id": 123}`)}, nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q := NewQueuedExecutor(mock, ctx)

	start := time.Now()
	resp, err := q.Request(&MessageConfig{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success on retry, got error: %v", err)
	}

	if resp == nil || !resp.Ok {
		t.Fatalf("expected Ok response, got %v", resp)
	}

	if elapsed < 1*time.Second {
		t.Errorf("expected at least 1s delay, got %v", elapsed)
	}

	mock.mu.Lock()
	if mock.calls != 2 {
		t.Errorf("expected 2 calls to Request, got %d", mock.calls)
	}
	mock.mu.Unlock()
}

func TestQueuedExecutor_ContextCancellation(t *testing.T) {
	mock := &mockQueueExecutor{
		onCall: func(n int) (*APIResponse, error) {
			return &APIResponse{
				Ok:        false,
				ErrorCode: 429,
				Parameters: &ResponseParameters{
					RetryAfter: 10, // long wait
				},
			}, &Error{Code: 429}
		},
	}

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	q := NewQueuedExecutor(mock, ctx)

	// Start a request in a goroutine
	errChan := make(chan error, 1)
	go func() {
		_, err := q.Request(&MessageConfig{})
		errChan <- err
	}()

	// Wait a bit to ensure it hits the rate limit and starts waiting
	time.Sleep(100 * time.Millisecond)

	// Cancel the context
	cancel()

	// Wait for the result
	select {
	case err := <-errChan:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for Request to return after cancellation")
	}
}
