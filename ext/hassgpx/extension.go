package hassgpx

import (
	"context"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/homebot/app"
	"github.com/jfk9w-go/homebot/core"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
)

var Extension app.Extension = extension{}

type extension struct{}

func (extension) ID() string {
	return "hassgpx"
}

func (extension) Apply(_ context.Context, app app.Interface, buttons *core.ControlButtons) (interface{}, error) {
	config := new(struct {
		HassGPX *struct {
			Database string
			Lookback flu.Duration
			Users    map[telegram.ID]string
		}
	})

	if err := app.GetConfig(config); err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	if config.HassGPX == nil {
		return nil, nil
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
		UserIDs:          config.HassGPX.Users,
		Lookback:       config.HassGPX.Lookback.GetOrDefault(24 * time.Hour),
	}, nil
}
