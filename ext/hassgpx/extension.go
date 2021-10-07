package hassgpx

import (
	"context"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/homebot/app"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
)

var Extension app.Extension = extension{}

type extension struct{}

func (extension) ID() string {
	return "hassgpx"
}

func (extension) Apply(_ context.Context, app app.Interface) (interface{}, error) {
	globalConfig := new(struct {
		HassGPX *struct {
			Database     string
			MaxSpeed     *float64
			LastDays     int
			MoveInterval flu.Duration
			Users        map[telegram.ID]string
		}
	})

	if err := app.GetConfig(globalConfig); err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	config := globalConfig.HassGPX
	if config == nil {
		return nil, nil
	}

	db, err := app.GetDatabase("postgres", config.Database)
	if err != nil {
		return nil, errors.Wrap(err, "get database")
	}

	maxSpeed := 55.
	if config.MaxSpeed != nil {
		maxSpeed = *config.MaxSpeed
	}

	storage := (*SQLStorage)(db)
	return &Service{
		Clock:        app,
		Storage:      storage,
		UserIDs:      config.Users,
		MaxSpeed:     maxSpeed,
		LastDays:     config.LastDays,
		MoveInterval: config.MoveInterval.GetOrDefault(time.Minute),
	}, nil
}
