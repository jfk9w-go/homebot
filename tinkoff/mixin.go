package tinkoff

import (
	"context"
	"time"

	"homebot/3rdparty/tinkoff"

	"github.com/jfk9w-go/flu/syncf"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/telegram-bot-api/ext"

	"github.com/jfk9w-go/flu/apfel"
	"github.com/jfk9w-go/flu/colf"
	"github.com/jfk9w-go/flu/logf"
	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/tapp"
	"github.com/pkg/errors"
)

type (
	Config struct {
		DB          apfel.GormConfig           `yaml:"db" doc:"This database will be used for saving bank data. Tables and views will be created automatically. Only 'postgres' driver is supported."`
		Credentials map[telegram.ID]Credential `yaml:"credentials" doc:"User credentials so you don't have to enter your password each time you want to sync data. Keys are telegram user IDs and values are credentials.\nOnly users with IDs found in this map will be allowed to execute /update_bank_statement (they still need to receive and enter confirmation code, though)."`
		Overlap     flu.Duration               `yaml:"overlap,omitempty" doc:"Minimum amount of data to be reloaded each time." default:"24h"`
	}

	Context interface {
		tapp.Context
		TinkoffConfig() Config
	}

	Mixin[C Context] struct {
		app         apfel.MixinApp[C]
		telegram    tapp.Mixin[C]
		storage     Storage[C]
		credentials map[telegram.ID]Credential
		overlap     time.Duration
	}
)

func (m Mixin[C]) String() string {
	return "tinkoff"
}

func (m *Mixin[C]) Include(ctx context.Context, app apfel.MixinApp[C]) error {
	if err := app.Use(ctx, &m.storage, false); err != nil {
		return err
	}

	if err := app.Use(ctx, &m.telegram, false); err != nil {
		return err
	}

	config := app.Config().TinkoffConfig()
	m.credentials = config.Credentials
	m.overlap = config.Overlap.Value

	m.app = app

	return nil
}

func (m *Mixin[C]) CommandScope() tapp.CommandScope {
	userIDs := make(colf.Set[telegram.ID], len(m.credentials))
	for userID := range m.credentials {
		userIDs.Add(userID)
	}

	return tapp.CommandScope{UserIDs: userIDs}
}

var defaultChapters = []chapter{
	tradingOperationsChapter{},
	accountsChapter{},
}

//goland:noinspection GoSnakeCaseUsage
func (m *Mixin[C]) Update_bank_statement(ctx context.Context, _ telegram.Client, cmd *telegram.Command) error {
	credential, ok := m.credentials[cmd.User.ID]
	if !ok {
		return errors.New("invalid user ID")
	}

	logf.Get(m).Debugf(ctx, "got credentials for [%s]", credential.Username)

	client := tinkoff.Client[C]{
		Credential: credential,
	}

	if err := m.app.Use(ctx, &client, false); err != nil {
		return err
	}

	html := ext.HTML(context.Background(), m.telegram.Bot(), cmd.User.ID)
	defer func() {
		if err := html.Flush(); err != nil {
			logf.Get(m).Errorf(ctx, "reply to [%s]: %v", credential.Username, err)
		}
	}()

	defer syncf.GoSync(ctx, func(ctx context.Context) {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := m.telegram.Bot().SendChatAction(ctx, cmd.Chat.ID, "typing"); err != nil {
					logf.Get(m).Warnf(ctx, "send typing action for [%s]: %v",
						credential.Username, err)
				}
			}
		}
	})()

	logger := newTelegramLogger(html)
	defer logger.flush()

	cvs := canvas{
		Client:           &client,
		StorageInterface: &m.storage,
		logger:           logger,
		username:         credential.Username,
		overlap:          m.overlap,
	}

	for _, chapter := range defaultChapters {
		sync(ctx, cvs, chapter)
	}

	return nil
}

func sync(ctx context.Context, cvs canvas, chapter chapter) {
	cvs.logger = cvs.sub(chapter.name())
	chapters, err := chapter.sync(ctx, &cvs)
	if err != nil {
		cvs.errorf(ctx, err.Error())
		return
	}

	for _, chapter := range chapters {
		sync(ctx, cvs, chapter)
	}
}
