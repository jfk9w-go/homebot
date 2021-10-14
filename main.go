package main

import (
	"homebot/hassgpx"
	"homebot/tinkoff"
	"homebot/tinkoff/sync"

	"github.com/jfk9w-go/telegram-bot-api/ext/app"
	"gorm.io/driver/postgres"
)

var GitCommit = "dev"

func main() {
	app.GormDialects["postgres"] = postgres.Open
	app.Run(GitCommit, "HOMEBOT_",
		tinkoff.Extension{sync.Accounts, sync.TradingOperations, sync.PurchasedSecurities},
		hassgpx.Extension)
}
