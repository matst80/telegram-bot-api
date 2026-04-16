package tgbotapi

import (
	"testing"
)

func TestCreateForumTopic(t *testing.T) {
	bot, _ := getBot(t)

	cfg := CreateForumTopicConfig{
		BaseChat: BaseChat{ChatID: SupergroupChatID},
		Name:     "Test Topic",
	}

	_, err := bot.CreateForumTopic(cfg)
	if err != nil {
		t.Error(err)
	}
}

func TestEditForumTopic(t *testing.T) {
	bot, _ := getBot(t)

	cfg := EditForumTopicConfig{
		BaseChat: BaseChat{
			ChatID:          SupergroupChatID,
			MessageThreadID: 1, // Dummy thread ID
		},
		Name: "Updated Topic Name",
	}

	_, err := bot.EditForumTopic(cfg)
	if err != nil {
		t.Error(err)
	}
}

func TestCloseForumTopic(t *testing.T) {
	bot, _ := getBot(t)

	cfg := CloseForumTopicConfig{
		BaseChat: BaseChat{
			ChatID:          SupergroupChatID,
			MessageThreadID: 1, // Dummy thread ID
		},
	}

	_, err := bot.CloseForumTopic(cfg)
	if err != nil {
		t.Error(err)
	}
}

func TestReopenForumTopic(t *testing.T) {
	bot, _ := getBot(t)

	cfg := ReopenForumTopicConfig{
		BaseChat: BaseChat{
			ChatID:          SupergroupChatID,
			MessageThreadID: 1, // Dummy thread ID
		},
	}

	_, err := bot.ReopenForumTopic(cfg)
	if err != nil {
		t.Error(err)
	}
}

func TestEditGeneralForumTopic(t *testing.T) {
	bot, _ := getBot(t)

	cfg := EditGeneralForumTopicConfig{
		ChatID: SupergroupChatID,
		Name:   "General Topic New Name",
	}

	_, err := bot.EditGeneralForumTopic(cfg)
	if err != nil {
		t.Error(err)
	}
}

func TestCloseGeneralForumTopic(t *testing.T) {
	bot, _ := getBot(t)

	cfg := CloseGeneralForumTopicConfig{
		ChatID: SupergroupChatID,
	}

	_, err := bot.CloseGeneralForumTopic(cfg)
	if err != nil {
		t.Error(err)
	}
}
