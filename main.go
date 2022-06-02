package main

import (
	"context"
	"os"
	"strings"

	"homebot/hassgpx"
	"homebot/tinkoff"

	"github.com/pkg/errors"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/apfel"
	"github.com/jfk9w-go/flu/gormf"
	"github.com/jfk9w-go/flu/logf"
	"github.com/jfk9w-go/telegram-bot-api"
	tg "github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/tapp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type C struct {
	Telegram tapp.Config      `yaml:"telegram" doc:"Telegram Bot API token."`
	Logging  apfel.LogfConfig `yaml:"logging,omitempty" doc:"Logging configuration."`
	HassGPX  struct {
		Enabled        bool `yaml:"enabled,omitempty" doc:"Enables the service and bot command."`
		hassgpx.Config `yaml:"-,inline"`
	} `yaml:"hassgpx,omitempty" doc:"HassGPX exposes a /get_gpx_track command which produces an auto-detected probably-bicycle GPX track based on Home Assistant tracking data over the last N days."`
	Tinkoff struct {
		Enabled        bool   `yaml:"enabled,omitempty" doc:"Enables the service and bot command."`
		Encode         string `yaml:"encode,omitempty" enum:"gob,yml,json" doc:"This will generate encoded credentials data from current config which can be piped to a separate config file and then used as '--config.file' CLI argument.\nThis is done for illusion of safety: you can remove encoded credentials from plain text config, and technically this is safer, but you should also take other reasonable precautions.\nExample: './homebot --config.file=config.yml --tinkoff.encode=gob > credentials.gob; ./homebot --config.file=config.yml --config.file=credentials.gob'"`
		tinkoff.Config `yaml:"-,inline"`
	} `yaml:"tinkoff,omitempty" doc:"Tinkoff exposes an /update_bank_statement command which pulls data from tinkoff.ru API and puts it into a database for further use."`
}

func (c C) TelegramConfig() tapp.Config   { return c.Telegram }
func (c C) LogfConfig() apfel.LogfConfig  { return c.Logging }
func (c C) HassGPXConfig() hassgpx.Config { return c.HassGPX.Config }
func (c C) TinkoffConfig() tinkoff.Config { return c.Tinkoff.Config }

const Description = `
  homebot is a sort-of-everyday (?) tool collection in the form of Telegram bot. At the moment it supports three commands:
    
    /start                 – replies with your user ID and bot version
                             This can be used to get user ID in order to fill tinkoff.credentials configuration section.

    /update_bank_statement – pulls data from tinkoff and puts it into a postgres
                             See 'tinkoff' configuration section for more info.
                             Note that it's recommended to encode credentials so as not to keep them in plain text configuration.

    /get_gpx_track         – collects Home Assistant tracking data from its database (only postgres supported)
                             in GPX format.
                             This uses some bold assumptions and rough approximations, you may want to check the code.
                             Also see 'hassgpx' configuration section for more info.
`

var GitCommit = "dev"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := apfel.Boot[C]{
		Name:    "homebot",
		Version: GitCommit,
		Desc:    Description,
	}.App(ctx)
	defer flu.CloseQuietly(app)

	if codec, ok := apfel.ExtensionCodecs[app.Config().Tinkoff.Encode]; ok {
		var config struct {
			Tinkoff struct {
				Credentials map[telegram.ID]tinkoff.Credential `yaml:"credentials"`
			} `yaml:"tinkoff"`
		}

		config.Tinkoff.Credentials = app.Config().Tinkoff.Credentials
		if err := flu.EncodeTo(codec(config), flu.IO{W: os.Stdout}); err != nil {
			logf.Get().Panicf(ctx, "encode tinkoff config: %+v", err)
		}

		return
	}

	var (
		gorm = apfel.Gorm[C]{
			Drivers: apfel.GormDrivers{
				"postgres": postgres.Open,
			},
			Config: gorm.Config{
				Logger: gormf.LogfLogger(app, func() logf.Interface { return logf.Get("gorm.sql") }),
			},
		}

		telegram tapp.Mixin[C]
	)

	app.Uses(ctx,
		new(apfel.Logf[C]),
		&gorm,
		&telegram,
	)

	if config := app.Config().Tinkoff; config.Enabled {
		app.Uses(ctx,
			&apfel.MixinAny[C, tinkoff.ConfirmFunc]{
				Value: func(ctx context.Context, username string) (code string, err error) {
					var userID tg.ID
					for uid, cred := range config.Credentials {
						if cred.Username == username {
							userID = uid
						}
					}

					if userID == 0 {
						return "", errors.Errorf("no telegram user ID matches Tinkoff username %s", username)
					}

					message, err := telegram.Bot().Ask(ctx, userID, &tg.Text{Text: "Enter SMS or push code"}, nil)
					if err != nil {
						return "", err
					}

					return strings.Trim(message.Text, " \n"), nil
				},
			},
			new(tinkoff.Mixin[C]),
		)
	}

	if config := app.Config().HassGPX; config.Enabled {
		app.Uses(ctx, new(hassgpx.Mixin[C]))
	}

	telegram.Run(ctx)
}
