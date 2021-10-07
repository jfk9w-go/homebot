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
}

func (s *Service) NewOutput() *output.Paged {
	return &output.Paged{
		Receiver: &receiver.Chat{
			Sender:    s.TgClient,
			ID:        s.ChatID,
			ParseMode: telegram.HTML,
		},
		PageSize: telegram.MaxMessageSize,
	}
}

func (s *Service) Gate() core.Gate {
	return core.Gate{
		ChatIDs: map[telegram.ChatID]bool{
			s.ChatID: true,
		},
	}
}
