package main

import (
	"context"
	"flag"
	"strings"
	"syscall"
	"time"

	"github.com/jfk9w-go/finbot/common"
	"github.com/jfk9w-go/finbot/tinkoff"
	"github.com/jfk9w-go/flu"
	fluhttp "github.com/jfk9w-go/flu/http"
	"github.com/sirupsen/logrus"
)

// Variables to be set while building.
var (
	GitCommit            = "dev"
	Tokens               = ""
	DatabaseURL          = ""
	WebDriverWaitTimeout = time.Minute
)

//noinspection GoUnhandledErrorResult
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:          true,
		DisableLevelTruncation: true,
	})

	logrus.Infof("built from commit %s", GitCommit)
	defer func() {
		if err := recover(); err != nil {
			logrus.Fatal(err)
		}
	}()

	databaseURL := flag.String("db", "", "Database URL")
	seleniumPath := flag.String("selenium", "/usr/local/Cellar/selenium-server-standalone/3.141.59_2/libexec/selenium-server-standalone-3.141.59.jar", "Selenium JAR path")
	webDriverPath := flag.String("webdriver", "/usr/local/bin/chromedriver", "Web driver path")
	pingIntervalStr := flag.String("ping-every", "1m", "Ping interval (as in time.Duration)")
	updateIntervalStr := flag.String("update-every", "20m", "Update interval (as in time.Duration)")
	flag.Parse()

	if databaseURL != nil && *databaseURL != "" {
		DatabaseURL = *databaseURL
	}

	if DatabaseURL == "" {
		logrus.Fatalf("db must be specified")
	}

	pingInterval, err := time.ParseDuration(*pingIntervalStr)
	if err != nil {
		logrus.Fatalf("ping-every has incorrect format: %s", err)
	}

	updateInterval, err := time.ParseDuration(*updateIntervalStr)
	if err != nil {
		logrus.Fatalf("update-every has incorrect format: %s", err)
	}

	db, err := common.NewDB(DatabaseURL, tinkoff.Tables...)
	if err != nil {
		logrus.Fatalf("create database connection: %s", err)
	}

	defer db.Close()

	httpClient := fluhttp.NewClient(nil)
	if Tokens != "" {
		logrus.Info("using predefined tokens")
		tokens := make(map[string]string)
		for _, pair := range strings.Split(Tokens, ";") {
			pairItems := strings.Split(strings.Trim(pair, " "), "=")
			username := pairItems[0]
			token := pairItems[1]
			tokens[username] = token
		}

		for username, token := range tokens {
			client := &tinkoff.Client{
				Auth:       tinkoff.SessionID(token),
				Username:   username,
				HttpClient: httpClient,
			}

			client.PingInBackground(ctx, pingInterval)
			defer client.Close()

			updater := &Updater{
				Client: client,
				DB:     db,
			}

			updater.RunInBackground(ctx, updateInterval)
			defer updater.Close()
		}
	} else {
		auth := &tinkoff.WebAuth{
			WebDriverConfig: common.WebDriverConfig{
				SeleniumPath: *seleniumPath,
				DriverPath:   *webDriverPath,
				WaitTimeout:  WebDriverWaitTimeout,
			},
			UserInput: common.BasicUserInput,
		}

		if _, err := auth.SessionID(); err != nil {
			logrus.Fatalf("web auth failed: %s", err)
		}

		client := &tinkoff.Client{
			Auth:       auth,
			Username:   auth.Username,
			HttpClient: httpClient,
		}

		client.PingInBackground(ctx, pingInterval)
		defer client.Close()

		updater := &Updater{
			Client: client,
			DB:     db,
		}

		updater.RunInBackground(ctx, updateInterval)
		defer updater.Close()
	}

	flu.AwaitSignal(syscall.SIGABRT, syscall.SIGINT, syscall.SIGKILL, syscall.SIGHUP, syscall.SIGTERM)
}

type Updater struct {
	DB     *common.DB
	Client *tinkoff.Client
	wg     *flu.WaitGroup
	cancel func()
}

func (u *Updater) RunInBackground(ctx context.Context, every time.Duration) {
	log := logrus.WithField("username", u.Client.Username)
	if u.wg != nil {
		log.Warnf("background update already running")
		return
	}

	u.wg = new(flu.WaitGroup)
	u.cancel = u.wg.Go(ctx, func(ctx context.Context) {
		ticker := time.NewTicker(every)
		defer func() {
			ticker.Stop()
			if err := ctx.Err(); err != nil {
				log.Warnf("update context canceled: %s", err)
			}
		}()

		now := time.Now()
		for {
			u.Update(ctx, log, now)
			if ctx.Err() != nil {
				return
			}

			select {
			case <-ctx.Done():
				return
			case now = <-ticker.C:
			}
		}
	})

	log.Infof("started background update")
}

func (u *Updater) Close() error {
	if u.wg != nil {
		u.cancel()
		u.wg.Wait()
	}

	return nil
}

func (u *Updater) Update(ctx context.Context, log *logrus.Entry, now time.Time) {
	accounts, err := u.Client.Accounts(ctx)
	if err != nil {
		log.Errorf("get accounts: %s", err)
	} else {
		importantAccounts := make([]tinkoff.Account, 0)
		for _, account := range accounts {
			if account.Type == "SharedCredit" ||
				account.Type == "SharedCurrent" ||
				account.Type == "ExternalAccount" {
				continue
			}

			importantAccounts = append(importantAccounts, account)
		}

		if err := u.DB.Update(ctx, importantAccounts); err != nil {
			log.Errorf("update accounts in db: %s", err)
		} else {
			log.Infof("updated %d accounts in db", len(importantAccounts))
		}

		for i := range importantAccounts {
			u.updateAccount(ctx, log, now, &importantAccounts[i])
		}
	}

	since, err := u.DB.UpdateSince(ctx, now, new(tinkoff.TradingOperation), u.Client.Username)
	if err != nil {
		log.Errorf("get update since for trading operations: %s", err)
	} else {
		tradingOperations, err := u.Client.TradingOperations(ctx, now, since)
		if err != nil {
			log.Errorf("get trading operations since %s: %s", since, err)
		} else if len(tradingOperations) > 0 {
			if err := u.DB.Update(ctx, tradingOperations); err != nil {
				log.Errorf("update trading operations in db: %s", err)
			} else {
				log.Infof("updated %d trading operations in db", len(tradingOperations))
			}
		}
	}

	securities, err := u.Client.PurchasedSecurities(ctx, now)
	if err != nil {
		log.Errorf("get purchased securities: %s", err)
	} else if err := u.DB.Update(ctx, securities); err != nil {
		log.Errorf("update purchased securities in db: %s", err)
	} else {
		log.Infof("updated %d purchased securities in db", len(securities))
	}

	log.Infof("update ok")
}

func (u *Updater) updateAccount(ctx context.Context, log *logrus.Entry, now time.Time, account *tinkoff.Account) {
	log = log.WithField("account", account)
	since, err := u.DB.UpdateSince(ctx, now, new(tinkoff.Operation), account.ID)
	if err != nil {
		log.Errorf("get update since for operations: %s", err)
	} else {
		operations, err := u.Client.Operations(ctx, now, account.ID, since)
		if err != nil {
			log.Errorf("get operations since %s: %s", since, err)
		} else if len(operations) > 0 {
			for i, operation := range operations {
				if operation.HasShoppingReceipt {
					receipt, err := u.Client.ShoppingReceipt(ctx, operation.ID)
					if err != nil {
						log.Errorf("get shopping receipt for %s: %s", operation.ID, err)
					} else {
						operations[i].ShoppingReceipt = receipt
					}
				}
			}

			if err := u.DB.Update(ctx, operations); err != nil {
				log.Errorf("update operations in db: %s", err)
			} else {
				log.Infof("updated %d operations in db", len(operations))
			}
		}
	}
}
