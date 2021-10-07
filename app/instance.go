package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/app"
	gormutil "github.com/jfk9w-go/flu/gorm"
	"github.com/jfk9w-go/homebot/core"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Instance struct {
	*app.Base
	extensions []Extension
	databases  map[string]*gorm.DB
	bot        *telegram.Bot
}

func Create(version string, clock flu.Clock, config flu.File) (*Instance, error) {
	base, err := app.New(version, clock, config, flu.YAML)
	if err != nil {
		return nil, err
	}

	return &Instance{
		Base:       base,
		extensions: make([]Extension, 0),
		databases:  make(map[string]*gorm.DB),
	}, nil
}

func (app *Instance) ApplyExtensions(extensions ...Extension) {
	app.extensions = append(app.extensions, extensions...)
}

func (app *Instance) GetDatabase(driver, conn string) (*gorm.DB, error) {
	if db, ok := app.databases[conn]; ok {
		return db, nil
	}

	var dialector gorm.Dialector
	switch driver {
	case "postgres":
		dialector = postgres.Open(conn)
	case "clickhouse":
		dialector = clickhouse.Open(conn)
	default:
		logrus.Fatalf("unknown dialect %s for %s", driver, conn)
	}

	db, err := gorm.Open(dialector, gormutil.DefaultConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "create database for %s", conn)
	}

	app.Manage((*gormutil.Closer)(db))
	app.databases[conn] = db
	return db, nil
}

func (app *Instance) GetBot(ctx context.Context) (*telegram.Bot, error) {
	if app.bot != nil {
		return app.bot, nil
	}

	config := new(struct{ Telegram struct{ Token string } })
	if err := app.GetConfig(config); err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	app.bot = telegram.NewBot(ctx, nil, config.Telegram.Token)
	return app.bot, nil
}

func (app *Instance) Run(ctx context.Context) error {
	registry := make(telegram.CommandRegistry)
	registry.AddFunc("/start", func(ctx context.Context, client telegram.Client, cmd *telegram.Command) error {
		return cmd.Reply(ctx, client, fmt.Sprintf("hi, %d @ %d", cmd.User.ID, cmd.Chat.ID))
	})

	scopeCommands := make(scopeCommands)
	for _, extension := range app.extensions {
		id := extension.ID()
		listener, err := extension.Apply(ctx, app)
		if err != nil {
			return errors.Wrapf(err, "apply plugin %s", id)
		}

		if listener != nil {
			commands := telegram.CommandRegistryFrom(listener)
			gated, ok := listener.(core.Gated)
			if ok {
				gate := gated.Gate()
				for key, listener := range commands {
					scopeCommands.addGated(gate, key)
					registry.Add(key, gate.Wrap(listener))
				}
			} else {
				for key, listener := range registry {
					scopeCommands.addDefault(key)
					registry.Add(key, listener)
				}
			}
		}

		logrus.WithField("service", id).Infof("init ok")
	}

	bot, err := app.GetBot(ctx)
	if err != nil {
		return errors.Wrap(err, "get bot")
	}

	if err := scopeCommands.set(ctx, bot); err != nil {
		return errors.Wrap(err, "set commands")
	}

	app.Manage(bot.CommandListener(registry))
	return nil
}

func humanizeKey(key string) string {
	return strings.Replace(strings.Title(strings.Trim(key, "/")), "_", " ", -1)
}
