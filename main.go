package main

import (
	"context"
	"fmt"
	"homebot/hassgpx"
	"homebot/tinkoff"
	"homebot/tinkoff/sync"

	"github.com/jfk9w-go/flu"
	fluapp "github.com/jfk9w-go/flu/app"
	tgapp "github.com/jfk9w-go/telegram-bot-api/ext/app"
	"github.com/pkg/errors"
	"gorm.io/driver/postgres"
)

var GitCommit = "dev"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fluapp.GormDialects["postgres"] = postgres.Open
	app := tgapp.Create(GitCommit, flu.DefaultClock)
	defer flu.CloseQuietly(app)
	app.ApplyExtensions(
		tinkoff.Extension{sync.Accounts, sync.TradingOperations, sync.PurchasedSecurities},
		hassgpx.Extension,
	)

	fluapp.Run(ctx, Instance{app}, fluapp.DefaultConfigurer("homebot"))
	flu.AwaitSignal()
}

type Instance struct {
	*tgapp.Instance
}

func (app Instance) Show() (bool, error) {
	if done, err := app.Instance.Show(); err != nil || done {
		return done, err
	}

	globalConfig := new(struct {
		Tinkoff struct {
			Show, Generate bool
			Data           flu.File
			Credentials    tinkoff.CredentialStore
		}
	})

	if err := app.GetConfig().As(globalConfig); err != nil {
		return false, errors.Wrap(err, "get config")
	}

	config := globalConfig.Tinkoff
	if config.Show {
		creds, err := tinkoff.DecodeCredentialsFrom(config.Data)
		if err != nil {
			return false, errors.Wrap(err, "decode credentials")
		}

		separator := "------------"
		println(separator)
		for key, cred := range creds {
			fmt.Printf("id: %d\nusername: %s\npassword: %s\n%s\n", key, cred.Username, cred.Password, separator)
		}

		return true, nil
	}

	if config.Generate {
		return true, errors.Wrap(config.Credentials.EncodeTo(config.Data), "generate")
	}

	return false, nil
}
