package surfaces

import (
	"context"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/homebot/app"
	"github.com/jfk9w-go/homebot/core"
	"github.com/jfk9w-go/homebot/ext/dooh"
	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
)

var Extension app.Extension = extension{}

type extension struct{}

func (extension) ID() string {
	return "dooh_surfaces"
}

func (extension) Apply(ctx context.Context, app app.Interface, buttons *core.ControlButtons) (interface{}, error) {
	globalConfig := new(struct {
		DOOH struct {
			Surfaces *struct {
				Data            flu.File
				Email, Password string
				CheckEvery      flu.Duration
			}

			ChatID telegram.ID
		}
	})

	if err := app.GetConfig(globalConfig); err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	chatID := globalConfig.DOOH.ChatID
	config := globalConfig.DOOH.Surfaces
	if config == nil {
		return nil, nil
	}

	bot, err := app.GetBot(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get bot")
	}

	checker := &Checker{
		Service: &dooh.Service{
			ChatID:   chatID,
			TgClient: bot,
			Buttons:  buttons,
		},
		ApiClient: NewApiClient(config.Email, config.Password),
		File:      config.Data,
	}

	if err := checker.RunInBackground(ctx, config.CheckEvery.GetOrDefault(time.Hour)); err != nil {
		return nil, errors.Wrap(err, "run in background")
	}

	app.Manage(checker)
	return checker, nil
}
