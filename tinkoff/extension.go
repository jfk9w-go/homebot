package tinkoff

import (
	"context"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/telegram-bot-api/ext/tapp"
	"github.com/pkg/errors"
)

type Extension func(receipts bool) []Executor

func (Extension) ID() string {
	return "tinkoff"
}

func (e Extension) Apply(ctx context.Context, app tapp.Application) (interface{}, error) {
	globalConfig := new(struct {
		Tinkoff struct {
			Enabled  bool
			Database string
			Data     flu.File
			Reload   flu.Duration
			Receipts bool
		}
	})

	if err := app.GetConfig().As(globalConfig); err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	config := globalConfig.Tinkoff
	if !config.Enabled {
		return nil, nil
	}

	db, err := app.GetDatabase("postgres", config.Database)
	if err != nil {
		return nil, errors.Wrap(err, "get database")
	}

	storage := (*SQLStorage)(db)
	if err := storage.Init(ctx); err != nil {
		return nil, errors.Wrap(err, "init storage")
	}

	creds, err := DecodeCredentialsFrom(config.Data)
	if err != nil {
		return nil, errors.Wrap(err, "decode creds")
	}

	return &Service{
		Context: &Context{
			Storage: storage,
		},
		Clock:       app,
		Credentials: creds,
		Executors:   e(config.Receipts),
	}, nil
}
