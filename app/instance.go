package app

import (
	"context"
	"fmt"

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
	buttons    *core.ControlButtons
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

func (app *Instance) GetControlButtons() *core.ControlButtons {
	if app.buttons == nil {
		app.buttons = core.NewControlButtons()
	}

	return app.buttons
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
	buttons := core.NewControlButtons()
	registry.AddFunc("/start", func(ctx context.Context, client telegram.Client, cmd *telegram.Command) error {
		output := buttons.Output(client, cmd)
		if err := output.WriteUnbreakable(ctx, fmt.Sprintf("hi, %d @ %d", cmd.User.ID, cmd.Chat.ID)); err != nil {
			return err
		}

		return output.Flush(ctx)
	})

	for _, extension := range app.extensions {
		id := extension.ID()
		listener, err := extension.Apply(ctx, app, buttons)
		if err != nil {
			return errors.Wrapf(err, "apply plugin %s", id)
		}

		if listener != nil {
			commands := telegram.CommandRegistryFrom(listener)
			gate, ok := listener.(core.Gate)
			if !ok {
				gate = core.Public
			}

			for key, command := range commands {
				if _, ok := registry[key]; ok {
					logrus.Fatalf("duplicate command handler for %s@%s", key, id)
				}

				registry[key] = core.ApplyGate(gate, command)
			}

			buttons.Add(commands, gate)
		}

		logrus.WithField("service", id).Infof("init ok")
	}

	bot, err := app.GetBot(ctx)
	if err != nil {
		return errors.Wrap(err, "get bot")
	}

	app.Manage(bot.CommandListener(registry))
	return nil
}
