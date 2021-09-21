package hassgpx

import (
	"context"
	"time"

	"github.com/jfk9w-go/flu"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"

	"github.com/jfk9w-go/homebot/app"
	"github.com/jfk9w-go/homebot/core"
)

var Extension app.Extension = extension{}

type extension struct{}

func (extension) Key() string {
	return "gpx"
}

func (extension) Icon() string {
	return "🗺"
}

func (extension) Apply(_ context.Context, app app.Interface, buttons *core.ControlButtons) (telegram.CommandListener, error) {
	config := new(struct {
		HassGPX struct {
			Database string
			Lookback flu.Duration
			Users    map[telegram.ID]string
		}
	})

	if err := app.GetConfig(config); err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	db, err := app.GetDatabase(config.HassGPX.Database)
	if err != nil {
		return nil, errors.Wrap(err, "get database")
	}

	storage := (*SQLStorage)(db)
	return &CommandListener{
		Clock:          app,
		Storage:        storage,
		ControlButtons: buttons,
		Users:          config.HassGPX.Users,
		Lookback:       config.HassGPX.Lookback.GetOrDefault(24 * time.Hour),
	}, nil
}
