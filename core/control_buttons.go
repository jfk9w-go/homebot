package core

import (
	"strings"

	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/output"
	"github.com/jfk9w-go/telegram-bot-api/ext/receiver"
)

type ControlButtons struct {
	buttons [][]telegram.Button
}

func NewControlButtons() *ControlButtons {
	return &ControlButtons{buttons: make([][]telegram.Button, 0)}
}

func (b *ControlButtons) Add(commands telegram.CommandRegistry) {
	buttons := make([]telegram.Button, len(commands))
	for key := range commands {
		buttons = append(buttons, (&telegram.Command{Key: key}).Button(humanizeKey(key)))
	}

	b.buttons = append(b.buttons, buttons)
}

func (b *ControlButtons) Output(client telegram.Client, cmd *telegram.Command) *output.Paged {
	return &output.Paged{
		Receiver: &receiver.Chat{
			Sender:      client,
			ID:          cmd.Chat.ID,
			ReplyMarkup: b.Keyboard(),
		},
		PageSize: telegram.MaxMessageSize,
	}
}

func (b *ControlButtons) Keyboard() telegram.ReplyMarkup {
	return telegram.InlineKeyboard(b.buttons...)
}

func humanizeKey(key string) string {
	return strings.Replace(strings.Title(strings.Trim(key, "/")), "_", " ", -1)
}
