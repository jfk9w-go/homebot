package main

import (
	"context"
	"homebot/hassgpx"
	"homebot/tinkoff"
	"homebot/tinkoff/sync"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/apfel"
	"github.com/jfk9w-go/telegram-bot-api/ext/tapp"
	"gorm.io/driver/postgres"
)

var GitCommit = "dev"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	apfel.GormDialects["postgres"] = postgres.Open
	app := Instance{tapp.Create(GitCommit, flu.DefaultClock)}
	defer flu.CloseQuietly(app)
	app.ApplyExtensions(
		tinkoff.Extension{sync.Accounts, sync.TradingOperations, sync.PurchasedSecurities, sync.Candles},
		hassgpx.Extension,
	)

	apfel.Run(ctx, app, apfel.DefaultConfigurer("homebot"))
}

type Instance struct {
	*tapp.Instance
}

func (app Instance) Aux() (bool, error) {
	if done, err := app.Instance.Aux(); err != nil || done {
		return done, err
	}

	return tinkoff.Run(app)
}
