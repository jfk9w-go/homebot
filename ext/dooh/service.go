package dooh

import (
	"github.com/jfk9w-go/homebot/core"
	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/output"
	"github.com/jfk9w-go/telegram-bot-api/ext/receiver"
)

type Service struct {
	ChatID   telegram.ID
	TgClient telegram.Client
	Buttons  *core.ControlButtons
}

func (s *Service) NewOutput() *output.Paged {
	return &output.Paged{
		Receiver: &receiver.Chat{
			Sender:      s.TgClient,
			ID:          s.ChatID,
			ParseMode:   telegram.HTML,
			ReplyMarkup: s.Buttons.Keyboard(s.ChatID, 0),
		},
		PageSize: telegram.MaxMessageSize,
	}
}

func (s *Service) Allow(chatID, userID telegram.ID) bool {
	return chatID == s.ChatID && userID != chatID
}
