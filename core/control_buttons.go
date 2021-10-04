package core

import (
	"context"
	"errors"
	"strings"

	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/output"
	"github.com/jfk9w-go/telegram-bot-api/ext/receiver"
)

type Gate interface {
	Allow(chatID, userID telegram.ID) bool
}

func ApplyGate(gate Gate, handler telegram.CommandListener) telegram.CommandListenerFunc {
	return func(ctx context.Context, client telegram.Client, cmd *telegram.Command) error {
		if gate.Allow(cmd.Chat.ID, cmd.User.ID) {
			return handler.OnCommand(ctx, client, cmd)
		}

		return errors.New("forbidden")
	}
}

type controlButtonsRow struct {
	buttons []telegram.Button
	gate    Gate
}

type ControlButtons struct {
	buttons []controlButtonsRow
}

func NewControlButtons() *ControlButtons {
	return &ControlButtons{buttons: make([]controlButtonsRow, 0)}
}

func (b *ControlButtons) Add(commands telegram.CommandRegistry, gate Gate) {
	buttons := make([]telegram.Button, len(commands))
	for key := range commands {
		buttons = append(buttons, (&telegram.Command{Key: key}).Button(humanizeKey(key)))
	}

	b.buttons = append(b.buttons, controlButtonsRow{buttons, gate})
}

func (b *ControlButtons) Output(client telegram.Client, cmd *telegram.Command) *output.Paged {
	return &output.Paged{
		Receiver: &receiver.Chat{
			Sender:      client,
			ID:          cmd.Chat.ID,
			ReplyMarkup: b.Keyboard(cmd.Chat.ID, cmd.User.ID),
		},
		PageSize: telegram.MaxMessageSize,
	}
}

func (b *ControlButtons) Keyboard(chatID, userID telegram.ID) telegram.ReplyMarkup {
	keyboard := make([][]telegram.Button, 0)
	for _, row := range b.buttons {
		if row.gate.Allow(chatID, userID) {
			keyboard = append(keyboard, row.buttons)
		}
	}

	return telegram.InlineKeyboard(keyboard...)
}

func humanizeKey(key string) string {
	return strings.Replace(strings.Title(strings.Trim(key, "/")), "_", " ", -1)
}
