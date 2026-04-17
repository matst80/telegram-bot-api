package tgbotapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type mockExecutor struct {
	updates []Update
}

func (m *mockExecutor) SetDebug(debug bool)               {}
func (m *mockExecutor) SetApiEndpoint(apiEndpoint string) {}
func (m *mockExecutor) Debug() bool {
	return false
}

func (m *mockExecutor) MakeRequest(endpoint string, params Params) (*APIResponse, error) {
	return m.MakeRequestWithContext(context.Background(), endpoint, params)
}
func (m *mockExecutor) MakeRequestWithContext(ctx context.Context, endpoint string, params Params) (*APIResponse, error) {
	if endpoint == "getMe" {
		user := User{ID: 1, UserName: "testbot"}
		data, _ := json.Marshal(user)
		return &APIResponse{Ok: true, Result: data}, nil
	}
	if endpoint == "getUpdates" {
		data, _ := json.Marshal(m.updates)
		m.updates = []Update{} // Clear after reading
		return &APIResponse{Ok: true, Result: data}, nil
	}
	return &APIResponse{Ok: true}, nil
}
func (m *mockExecutor) Request(c Chattable) (*APIResponse, error) {
	return m.RequestWithContext(context.Background(), c)
}
func (m *mockExecutor) RequestWithContext(ctx context.Context, c Chattable) (*APIResponse, error) {
	params, _ := c.params()
	return m.MakeRequestWithContext(ctx, c.method(), params)
}
func (m *mockExecutor) UploadFiles(endpoint string, params Params, files []RequestFile) (*APIResponse, error) {
	return &APIResponse{Ok: true}, nil
}

func TestNewMultipleListenerBotAPI(t *testing.T) {
	executor := &mockExecutor{}
	bot := &BotAPI{
		Token:           "token",
		Executor:        executor,
		Buffer:          100,
		shutdownChannel: make(chan interface{}),
	}
	// We need to set bot.Self because NewBotAPI (which normally does it) calls getMe
	bot.Self = User{ID: 1, UserName: "testbot"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewMultipleListenerBotAPI(ctx, bot, 1)

	received := make(chan Update, 1)
	handler := func(u Update) {
		received <- u
	}

	matcher := func(u Update) bool {
		return u.Message != nil && u.Message.Text == "hello"
	}

	s.AddListener(ctx, matcher, handler)

	update := Update{
		Message: &Message{
			Text: "hello",
		},
	}

	executor.updates = append(executor.updates, update)

	select {
	case u := <-received:
		if u.Message.Text != "hello" {
			t.Errorf("Expected hello, got %s", u.Message.Text)
		}
	case <-time.After(time.Second * 5):
		t.Error("Timed out waiting for update")
	}
}

func TestMultipleListenerBotAPI_RemoveListener(t *testing.T) {
	// This test might fail if AddListener returns a copy and RemoveListener expects the original pointer.
	// We'll see.
	executor := &mockExecutor{}
	bot := &BotAPI{
		Token:           "token",
		Executor:        executor,
		Buffer:          100,
		shutdownChannel: make(chan interface{}),
	}
	bot.Self = User{ID: 1, UserName: "testbot"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewMultipleListenerBotAPI(ctx, bot, 1)

	received := make(chan Update, 1)
	handler := func(u Update) {
		received <- u
	}

	matcher := func(u Update) bool {
		return true
	}

	// If AddListener returns by value, we can't easily get the pointer inside the slice.
	// But let's see what happens.
	l := s.AddListener(ctx, matcher, handler)
	s.RemoveListener(l)

	if len(s.listeners) != 0 {
		t.Errorf("Expected 0 listeners, got %d", len(s.listeners))
	}

	update := Update{
		Message: &Message{
			Text: "hello",
		},
	}

	executor.updates = append(executor.updates, update)

	select {
	case <-received:
		t.Error("Expected no update after removing listener")
	case <-time.After(time.Millisecond * 500):
		// Success
	}
}

func TestMultipleListenerBotAPI_ContextCancellation(t *testing.T) {
	executor := &mockExecutor{}
	bot := &BotAPI{
		Token:           "token",
		Executor:        executor,
		Buffer:          100,
		shutdownChannel: make(chan interface{}),
	}
	bot.Self = User{ID: 1, UserName: "testbot"}

	ctx, cancel := context.WithCancel(context.Background())
	s := NewMultipleListenerBotAPI(ctx, bot, 1)

	cancel() // Cancel immediately

	// Give it a moment to shut down
	time.Sleep(time.Millisecond * 100)

	received := make(chan Update, 1)
	s.AddListener(context.Background(), func(u Update) bool { return true }, func(u Update) {
		received <- u
	})

	executor.updates = append(executor.updates, Update{Message: &Message{Text: "after cancel"}})

	select {
	case <-received:
		t.Error("Listener received update after context cancellation")
	case <-time.After(time.Second * 1):
		// Success
	}
}
