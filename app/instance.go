package app

import (
	"context"

	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/app"
	gormutil "github.com/jfk9w-go/flu/gorm"

	"github.com/jfk9w-go/homebot/core"
)

type Instance struct {
	*app.Base
	extensions []Extension
	databases  map[string]*gorm.DB
	buttons    *core.ControlButtons
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

func (app *Instance) GetDatabase(conn string) (*gorm.DB, error) {
	if db, ok := app.databases[conn]; ok {
		return db, nil
	}

	db, err := gormutil.NewPostgres(conn)
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

func (app *Instance) Run(ctx context.Context) error {
	config := new(struct{ Telegram struct{ Token string } })
	if err := app.GetConfig(config); err != nil {
		return errors.Wrap(err, "get config")
	}

	registry := make(telegram.CommandRegistry)
	buttons := core.NewControlButtons()
	registry.AddFunc("/start", func(ctx context.Context, client telegram.Client, cmd *telegram.Command) error {
		output := buttons.Output(client, cmd)
		if err := output.WriteUnbreakable(ctx, "hi"); err != nil {
			return err
		}
		return output.Flush(ctx)
	})

	for _, plugin := range app.extensions {
		key := plugin.Key()
		if _, ok := registry[key]; ok {
			return errors.Errorf("extension already registered for key %s", key)
		}

		var err error
		registry[key], err = plugin.Apply(ctx, app, buttons)
		if err != nil {
			return errors.Wrapf(err, "apply plugin %s", key)
		}

		buttons.Add(plugin.Icon(), key)
		logrus.WithField("service", key).Infof("init ok")
	}

	bot := telegram.NewBot(ctx, nil, config.Telegram.Token)
	app.Manage(bot.CommandListener(registry))
	return nil
}
