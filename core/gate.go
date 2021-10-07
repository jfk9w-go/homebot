package core

import (
	"context"

	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
)

type Gated interface {
	Gate() Gate
}

type Gate struct {
	UserIDs map[telegram.ID]bool
	ChatIDs map[telegram.ChatID]bool
}

func (g Gate) Wrap(listener telegram.CommandListener) telegram.CommandListenerFunc {
	return func(ctx context.Context, client telegram.Client, cmd *telegram.Command) error {
		if g.allow(cmd.Chat.ID, cmd.User.ID) {
			return listener.OnCommand(ctx, client, cmd)
		}

		return errors.New("forbidden")
	}
}

func (g Gate) allow(chatID telegram.ChatID, userID telegram.ID) bool {
	if g.UserIDs != nil {
		return userID == chatID && g.UserIDs[userID]
	}

	if g.ChatIDs != nil {
		return userID != chatID && g.ChatIDs[chatID]
	}

	return false
}
