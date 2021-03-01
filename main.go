package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jfk9w-go/bank-statement/tinkoff"
	"github.com/jfk9w-go/flu"
	fluhttp "github.com/jfk9w-go/flu/http"
	"github.com/jfk9w-go/flu/serde"
	"github.com/pkg/errors"
)

type Config struct {
	DBConnection  string         `yaml:"db_connection"`
	SeleniumPath  string         `yaml:"selenium_path"`
	WebDriverPath string         `yaml:"web_driver_path"`
	WaitTimeout   serde.Duration `yaml:"wait_timeout"`
}

//noinspection GoUnhandledErrorResult
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var config Config
	if err := flu.DecodeFrom(flu.File(os.Args[1]), flu.YAML{Value: &config}); err != nil {
		panic(err)
	}

	db, err := tinkoff.NewDatabase(config.DBConnection, tinkoff.Tables...)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	var start time.Time
	now := time.Now()

	auth := &tinkoff.WebAuth{
		WebDriverConfig: tinkoff.WebDriverConfig{
			SeleniumPath: config.SeleniumPath,
			DriverPath:   config.WebDriverPath,
			WaitTimeout:  config.WaitTimeout.Duration,
		},
		TwoFactor: tinkoff.BasicTwoFactor,
	}

	_, err = auth.SessionID()
	if err != nil {
		panic(err)
	}

	client := &tinkoff.Client{
		HttpClient: fluhttp.NewClient(nil),
		Auth:       auth,
	}

	log.Printf("Loading accounts for %s", auth.Username)
	accounts, err := client.Accounts(ctx)
	if err != nil {
		panic(err)
	}

	for _, account := range accounts {
		if account.Type == "ExternalAccount" {
			continue
		}

		log.Printf("Loading operations for %s (%s)", account.Name, account.ID)
		operations, err := client.Operations(ctx, account.ID, start, now)
		if err != nil {
			panic(err)
		}

		for i, operation := range operations {
			if operation.HasShoppingReceipt {
				log.Printf("Loading shopping receipt for %d", operation.ID)
				receipt, err := client.ShoppingReceipt(ctx, operation.ID)
				if err != nil {
					panic(errors.Wrapf(err, "%+v", operation))
				}

				operations[i].ShoppingReceipt = receipt
			}
		}

		if err := db.Update(ctx, operations); err != nil {
			panic(err)
		}

		log.Printf("Updated account %s", account.Name)
	}

	tradingOperations, err := client.TradingOperations(ctx, start, now)
	if err != nil {
		panic(err)
	}

	for i := range tradingOperations {
		tradingOperations[i].Username = auth.Username
	}

	if err := db.Update(ctx, tradingOperations); err != nil {
		panic(err)
	}

	log.Printf("Finished updating trading operations")
}
