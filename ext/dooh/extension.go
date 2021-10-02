package dooh

import (
	"context"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/homebot/app"
	"github.com/jfk9w-go/homebot/core"
	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
)

var Extension app.Extension = extension{}

type extension struct{}

func (extension) ID() string {
	return "dooh"
}

func (extension) Apply(ctx context.Context, app app.Interface, buttons *core.ControlButtons) (interface{}, error) {
	config := new(struct {
		DOOH struct {
			Data            flu.File
			UpdateEvery     flu.Duration
			Email, Password string
			ChatID          telegram.ID
		}
	})

	if err := app.GetConfig(config); err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	bot, err := app.GetBot(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get bot")
	}

	service := &Service{
		Client:         NewClient(config.DOOH.Email, config.DOOH.Password),
		TelegramClient: bot,
		File:           config.DOOH.Data,
		ChatID:         config.DOOH.ChatID,
	}

	if err := service.RunInBackground(ctx, config.DOOH.UpdateEvery.GetOrDefault(time.Hour)); err != nil {
		return nil, errors.Wrap(err, "run in background")
	}

	app.Manage(service)
	return nil, nil
}
