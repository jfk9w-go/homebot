package reports

import (
	"context"

	"github.com/jfk9w-go/homebot/app"
	"github.com/jfk9w-go/homebot/core"
	"github.com/jfk9w-go/homebot/ext/dooh"
	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
)

var Extension app.Extension = extension{}

type extension struct{}

func (extension) ID() string {
	return "dooh_reports"
}

func (extension) Apply(ctx context.Context, app app.Interface, buttons *core.ControlButtons) (interface{}, error) {
	globalConfig := new(struct {
		DOOH struct {
			Reports *struct {
				Token      string
				Clickhouse string
				At         string
				LastDays   int
				Thresholds Thresholds
			}

			ChatID telegram.ID
		}
	})

	if err := app.GetConfig(globalConfig); err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	chatID := globalConfig.DOOH.ChatID
	config := globalConfig.DOOH.Reports
	if config == nil {
		return nil, nil
	}

	bot, err := app.GetBot(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get bot")
	}

	db, err := app.GetDatabase("clickhouse", config.Clickhouse)
	if err != nil {
		return nil, errors.Wrap(err, "get database")
	}

	if config.LastDays < 1 {
		config.LastDays = 14
	}

	checker := &Checker{
		Service: &dooh.Service{
			ChatID:   chatID,
			TgClient: bot,
			Buttons:  buttons,
		},
		Clock:          app,
		QueryApiClient: NewQueryApiClient(config.Token),
		Clickhouse:     db,
		LastDays:       config.LastDays,
		Thresholds:     config.Thresholds,
	}

	if err := checker.RunInBackground(ctx, config.At); err != nil {
		return nil, errors.Wrap(err, "run in background")
	}

	app.Manage(checker)
	return checker, nil
}
