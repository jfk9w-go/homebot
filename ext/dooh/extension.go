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
	globalConfig := new(struct {
		DOOH *struct {
			Data            flu.File
			UpdateEvery     flu.Duration
			Email, Password string
			ChatID          telegram.ID
		}
	})

	if err := app.GetConfig(globalConfig); err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	config := globalConfig.DOOH
	if config == nil {
		return nil, nil
	}

	bot, err := app.GetBot(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get bot")
	}

	listener := &CommandListener{
		Client:         NewClient(config.Email, config.Password),
		TelegramClient: bot,
		File:           config.Data,
		ChatID:         config.ChatID,
		ControlButtons: buttons,
	}

	if err := listener.RunInBackground(ctx, config.UpdateEvery.GetOrDefault(time.Hour)); err != nil {
		return nil, errors.Wrap(err, "run in background")
	}

	app.Manage(listener)
	return listener, nil
}
