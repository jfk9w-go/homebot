package core

import (
	"context"

	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
)

type Gate interface {
	Allow(chatID, userID telegram.ID) bool
}

var Public Gate = public{}

type public struct{}

func (public) Allow(chatID, userID telegram.ID) bool {
	return true
}

func ApplyGate(gate Gate, handler telegram.CommandListener) telegram.CommandListenerFunc {
	return func(ctx context.Context, client telegram.Client, cmd *telegram.Command) error {
		if gate.Allow(cmd.Chat.ID, cmd.User.ID) {
			return handler.OnCommand(ctx, client, cmd)
		}

		return errors.New("forbidden")
	}
}
