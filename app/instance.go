package app

import (
	"context"

	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/app"
	gormutil "github.com/jfk9w-go/flu/gorm"
)

type Instance struct {
	*app.Base
	plugins   []Extension
	databases map[string]*gorm.DB
}

func Create(version string, clock flu.Clock, config flu.File) (*Instance, error) {
	base, err := app.New(version, clock, config, flu.YAML)
	if err != nil {
		return nil, err
	}

	return &Instance{
		Base:      base,
		plugins:   make([]Extension, 0),
		databases: make(map[string]*gorm.DB),
	}, nil
}

func (app *Instance) ApplyPlugins(plugins ...Extension) {
	app.plugins = append(app.plugins, plugins...)
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

func (app *Instance) Run(ctx context.Context) error {
	config := new(struct{ Telegram struct{ Token string } })
	if err := app.GetConfig(config); err != nil {
		return errors.Wrap(err, "get config")
	}

	bot := telegram.NewBot(ctx, nil, config.Telegram.Token)
	registry := make(telegram.CommandRegistry)
	for _, plugin := range app.plugins {
		key := plugin.Key()
		if _, ok := registry[key]; ok {
			return errors.Errorf("plugin already registered for key %s", key)
		}

		var err error
		registry[key], err = plugin.Apply(ctx, app)
		if err != nil {
			return errors.Wrapf(err, "apply plugin %s", key)
		}

		logrus.WithField("service", key).Infof("init ok")
	}

	app.Manage(bot.CommandListener(registry))
	return nil
}
