package tinkoff

import (
	"context"
	"time"

	"github.com/jfk9w-go/flu"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/tapp"
	"github.com/pkg/errors"
)

type Extension []Executor

func (e Extension) ID() string {
	return "tinkoff"
}

func (e Extension) Buttons() []telegram.Button {
	return []telegram.Button{
		(&telegram.Command{Key: "/tsync"}).Button("Update bank data"),
	}
}

func (e Extension) Apply(ctx context.Context, app tapp.Application) (interface{}, error) {
	globalConfig := new(struct {
		Tinkoff struct {
			Enabled  bool
			Database string
			Data     flu.File
			Reload   flu.Duration
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
			Reload:  config.Reload.GetOrDefault(60 * 24 * time.Hour),
		},
		Clock:       app,
		Credentials: creds,
		Executors:   e,
	}, nil
}
