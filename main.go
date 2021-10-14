package main

import (
	"homebot/hassgpx"
	"homebot/tinkoff"
	"homebot/tinkoff/sync"

	"github.com/jfk9w-go/flu"
	tgapp "github.com/jfk9w-go/telegram-bot-api/ext/app"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
)

var GitCommit = "dev"

func main() {
	tgapp.GormDialects["postgres"] = postgres.Open
	configurer := tgapp.DefaultConfigurer("HOMEBOT_", nil, "config.file", "config.stdin")
	app, err := tgapp.Create(GitCommit, flu.DefaultClock, configurer)
	if err != nil {
		logrus.Fatal(err)
	}

	defer flu.CloseQuietly(app)
	if ok, err := app.Show(); err != nil {
		logrus.Fatal(err)
	} else if ok {
		return
	}

	if ok, err := tinkoff.Run(app); err != nil {
		logrus.Fatal(err)
	} else if ok {
		return
	}

	tgapp.Run(app,
		tinkoff.Extension{sync.Accounts, sync.TradingOperations, sync.PurchasedSecurities},
		hassgpx.Extension,
	)
}
