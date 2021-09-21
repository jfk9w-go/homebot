package core

import (
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/output"
	"github.com/jfk9w-go/telegram-bot-api/ext/receiver"
)

type ControlButtons struct {
	buttons []telegram.Button
}

func NewControlButtons() *ControlButtons {
	return &ControlButtons{buttons: make([]telegram.Button, 0)}
}

func (b *ControlButtons) Add(text, key string) {
	b.buttons = append(b.buttons, (&telegram.Command{Key: key}).Button(text))
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
	return telegram.InlineKeyboard(b.buttons)
}
