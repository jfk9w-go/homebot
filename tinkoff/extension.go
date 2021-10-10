package tinkoff

import (
	"context"
	"time"

	"github.com/jfk9w-go/flu"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/app"
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

func (e Extension) Apply(ctx context.Context, app app.Interface) (interface{}, error) {
	config := new(struct {
		Tinkoff *struct {
			Database string
			Data     string
			Reload   flu.Duration
		}
	})

	if err := app.GetConfig(config); err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	if config.Tinkoff == nil {
		return nil, nil
	}

	db, err := app.GetDatabase("postgres", config.Tinkoff.Database)
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

	return &Service{
		Context: &Context{
			Storage: storage,
			Reload:  config.Tinkoff.Reload.GetOrDefault(60 * 24 * time.Hour),
		},
		Clock:       app,
		Credentials: creds,
		Executors:   e,
	}, nil
}