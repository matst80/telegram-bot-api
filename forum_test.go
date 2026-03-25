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
