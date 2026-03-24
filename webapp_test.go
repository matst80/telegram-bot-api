package tgbotapi

import (
	"encoding/json"
	"testing"
)

func TestUsersShared(t *testing.T) {
	data := `{"users_shared":{"request_id":1,"user_ids":[123,456]}}`
	var msg Message
	err := json.Unmarshal([]byte(data), &msg)
	if err != nil {
		t.Fatal(err)
	}

	if msg.UsersShared == nil {
		t.Fatal("UsersShared should not be nil")
	}

	if msg.UsersShared.RequestID != 1 {
		t.Errorf("expected request_id 1, got %d", msg.UsersShared.RequestID)
	}

	if len(msg.UsersShared.UserIDs) != 2 || msg.UsersShared.UserIDs[0] != 123 {
		t.Errorf("expected user_ids [123, 456], got %v", msg.UsersShared.UserIDs)
	}
}

func TestChatShared(t *testing.T) {
	data := `{"chat_shared":{"request_id":2,"chat_id":789}}`
	var msg Message
	err := json.Unmarshal([]byte(data), &msg)
	if err != nil {
		t.Fatal(err)
	}

	if msg.ChatShared == nil {
		t.Fatal("ChatShared should not be nil")
	}

	if msg.ChatShared.RequestID != 2 {
		t.Errorf("expected request_id 2, got %d", msg.ChatShared.RequestID)
	}

	if msg.ChatShared.ChatID != 789 {
		t.Errorf("expected chat_id 789, got %d", msg.ChatShared.ChatID)
	}
}

func TestWriteAccessAllowed(t *testing.T) {
	data := `{"write_access_allowed":{"from_request":true,"web_app_name":"TestApp"}}`
	var msg Message
	err := json.Unmarshal([]byte(data), &msg)
	if err != nil {
		t.Fatal(err)
	}

	if msg.WriteAccessAllowed == nil {
		t.Fatal("WriteAccessAllowed should not be nil")
	}

	if !msg.WriteAccessAllowed.FromRequest {
		t.Error("expected from_request true")
	}

	if msg.WriteAccessAllowed.WebAppName != "TestApp" {
		t.Errorf("expected web_app_name TestApp, got %s", msg.WriteAccessAllowed.WebAppName)
	}
}

func TestKeyboardButtonRequestUsers(t *testing.T) {
	isBot := true
	btn := KeyboardButton{
		Text: "Request Bots",
		RequestUsers: &KeyboardButtonRequestUsers{
			RequestID: 1,
			UserIsBot: &isBot,
		},
	}

	data, err := json.Marshal(btn)
	if err != nil {
		t.Fatal(err)
	}

	expected := `{"text":"Request Bots","request_users":{"request_id":1,"user_is_bot":true}}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}
}

func TestMenuButtonHelpers(t *testing.T) {
	commands := NewMenuButtonCommands()
	if commands.Type != "commands" {
		t.Errorf("expected type commands, got %s", commands.Type)
	}

	webapp := NewMenuButtonWebApp("Open Web App", WebAppInfo{URL: "https://example.com"})
	if webapp.Type != "web_app" || webapp.Text != "Open Web App" || webapp.WebApp.URL != "https://example.com" {
		t.Errorf("invalid MenuButtonWebApp: %+v", webapp)
	}

	def := NewMenuButtonDefault()
	if def.Type != "default" {
		t.Errorf("expected type default, got %s", def.Type)
	}
}
