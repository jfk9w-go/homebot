package tinkoff

import (
	"context"
	"strconv"
	"strings"
	"time"

	"homebot/tinkoff/external"

	"github.com/jfk9w-go/flu/colf"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/apfel"
	"github.com/jfk9w-go/flu/logf"
	"github.com/jfk9w-go/flu/syncf"
	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext"
	"github.com/jfk9w-go/telegram-bot-api/ext/tapp"
	"github.com/pkg/errors"
)

type (
	Credential = external.Credential

	Config struct {
		Enabled     bool                       `yaml:"enabled,omitempty" doc:"Enables the service and bot command."`
		DB          apfel.GormConfig           `yaml:"db" doc:"This database will be used for saving bank data. Tables and views will be created automatically. Only 'postgres' driver is supported."`
		Credentials map[telegram.ID]Credential `yaml:"credentials" doc:"User credentials so you don't have to enter your password each time you want to sync data. Keys are telegram user IDs and values are credentials.\nOnly users with IDs found in this map will be allowed to execute /update_bank_statement (they still need to receive and enter confirmation code, though)."`
		Reload      flu.Duration               `yaml:"reload,omitempty" doc:"Default time interval to synchronize data for (the point of origin may differ for different chapters, or it may be not used at all).\nThis can be overridden by the first parameter to /update_bank_statement [days int]." example:"72h" default:"168h" format:"duration"`
		Receipts    bool                       `yaml:"receipts,omitempty" doc:"Whether to enable shopping receipt downloading. It is known to hit rate limits recently, which cool down pretty slowly, so enable at your own risk (there is none, really)."`
		Chapters    colf.Set[string]           `yaml:"chapters,omitempty" doc:"Object chapters to synchronize. You can find chapter explanation in README.md (though the titles should be pretty self-explanatory)." enum:"tinkoff.chapter.accounts,tinkoff.chapter.trading-operations,tinkoff.chapter.purchased-securities,tinkoff.chapter.candles" default:"[\"tinkoff.chapter.accounts\",\"tinkoff.chapter.trading-operations\",\"tinkoff.chapter.purchased-securities\",\"tinkoff.chapter.candles\"]"`
	}

	Context interface{ TinkoffConfig() Config }

	Chapter interface {
		Title() string
		Sync(ctx context.Context, client *external.Client, period time.Duration) ([]Chapter, int, error)
	}

	Mixin[C Context] struct {
		clock       syncf.Clock
		credentials map[telegram.ID]Credential
		chapters    []Chapter
	}
)

func (m *Mixin[C]) String() string {
	return "tinkoff"
}

func (m *Mixin[C]) TelegramListener() tapp.Listener {
	return m
}

func (m *Mixin[C]) Include(ctx context.Context, app apfel.MixinApp[C]) error {
	config := app.Config().TinkoffConfig()
	if !config.Enabled {
		return apfel.ErrDisabled
	}

	m.clock = app
	m.credentials = config.Credentials

	return nil
}

func (m *Mixin[C]) AfterInclude(ctx context.Context, app apfel.MixinApp[C], mixin apfel.Mixin[C]) error {
	if chapter, ok := mixin.(Chapter); ok {
		m.chapters = append(m.chapters, chapter)
		logf.Get(m).Infof(ctx, "registered chapter: %s", chapter)
	}

	return nil
}

func (m *Mixin[C]) CommandScope() tapp.CommandScope {
	userIDs := make(colf.Set[telegram.ID], len(m.credentials))
	for userID := range m.credentials {
		userIDs.Add(userID)
	}

	return tapp.CommandScope{UserIDs: userIDs}
}

//goland:noinspection GoSnakeCaseUsage
func (m *Mixin[C]) Update_bank_statement(ctx context.Context, tgclient telegram.Client, cmd *telegram.Command) error {
	credential := m.credentials[cmd.User.ID]
	client, err := external.Authorize(ctx, credential, func(ctx context.Context) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		m, err := tgclient.Ask(ctx, cmd.Chat.ID, &telegram.Text{Text: "Enter SMS or push code"}, nil)
		if err != nil {
			return "", err
		}

		return strings.Trim(m.Text, " \n"), nil
	})

	if err != nil {
		return err
	}

	logf.Get(client).Debugf(ctx, "authorized successfully")

	period := 60 * 24 * time.Hour
	if daysStr := cmd.Arg(0); daysStr != "" {
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return errors.Wrap(err, "parse days")
		}

		period = time.Duration(days) * 24 * time.Hour
	}

	html := ext.HTML(ctx, tgclient, cmd.Chat.ID)
	for _, chapter := range m.chapters {
		sync := chapterSync{
			client:  client,
			chapter: chapter,
			period:  period,
			log:     func() logf.Interface { return logf.Get(client) },
		}

		if err := sync.run(ctx, html); err != nil {
			return err
		}
	}

	return html.Flush()
}
