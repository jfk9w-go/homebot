package core

import (
	"strings"

	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/output"
	"github.com/jfk9w-go/telegram-bot-api/ext/receiver"
)

type controlButtonsRow struct {
	buttons []telegram.Button
	userIDs map[telegram.ID]bool
	chatIDs map[telegram.ID]bool
}

type ControlButtons struct {
	buttons []controlButtonsRow
}

func NewControlButtons() *ControlButtons {
	return &ControlButtons{buttons: make([]controlButtonsRow, 0)}
}

func (b *ControlButtons) Add(commands telegram.CommandRegistry, userIDs map[telegram.ID]bool, chatIDs map[telegram.ID]bool) {
	buttons := make([]telegram.Button, len(commands))
	for key := range commands {
		buttons = append(buttons, (&telegram.Command{Key: key}).Button(humanizeKey(key)))
	}

	b.buttons = append(b.buttons, controlButtonsRow{buttons, userIDs, chatIDs})
}

func (b *ControlButtons) Output(client telegram.Client, cmd *telegram.Command) *output.Paged {
	return &output.Paged{
		Receiver: &receiver.Chat{
			Sender:      client,
			ID:          cmd.Chat.ID,
			ReplyMarkup: b.Keyboard(cmd.User.ID, cmd.User.ID),
		},
		PageSize: telegram.MaxMessageSize,
	}
}

func (b *ControlButtons) Keyboard(userID, chatID telegram.ID) telegram.ReplyMarkup {
	keyboard := make([][]telegram.Button, 0)
	for _, row := range b.buttons {
		if (row.userIDs == nil || userID == chatID && row.userIDs[userID]) && (row.chatIDs == nil || userID != chatID && row.chatIDs[chatID]) {
			keyboard = append(keyboard, row.buttons)
		}
	}

	return telegram.InlineKeyboard(keyboard...)
}

func humanizeKey(key string) string {
	return strings.Replace(strings.Title(strings.Trim(key, "/")), "_", " ", -1)
}
