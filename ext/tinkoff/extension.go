package tinkoff

import (
	"context"
	"time"

	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"

	"github.com/jfk9w-go/flu"

	"github.com/jfk9w-go/homebot/app"
)

type Extension []Executor

func (e Extension) Key() string {
	return "/t"
}

func (e Extension) Apply(ctx context.Context, app app.Interface) (telegram.CommandListener, error) {
	config := new(struct {
		Tinkoff struct {
			Database string
			Data     string
			Reload   flu.Duration
		}
	})

	if err := app.GetConfig(config); err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	db, err := app.GetDatabase(config.Tinkoff.Database)
	if err != nil {
		return nil, errors.Wrap(err, "get database")
	}

	storage := (*SQLStorage)(db)
	if err := storage.Init(ctx); err != nil {
		return nil, errors.Wrap(err, "init storage")
	}

	creds := make(CredentialStore)
	if err := flu.DecodeFrom(flu.File(config.Tinkoff.Data), flu.Gob(&creds)); err != nil {
		return nil, errors.Wrap(err, "decode creds")
	}

	return &CommandListener{
		Context: &Context{
			Storage: storage,
			Reload:  config.Tinkoff.Reload.GetOrDefault(60 * 24 * time.Hour),
		},
		Clock:       app,
		Credentials: creds,
		Executors:   e,
	}, nil
}
