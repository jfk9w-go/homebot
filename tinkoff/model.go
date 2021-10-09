package tinkoff

import (
	"context"
	"time"

	"homebot/tinkoff/external"

	"github.com/jfk9w-go/flu"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/output"
)

type Credential struct {
	Username, Password string
}

type CredentialStore map[telegram.ID]Credential

type Storage interface {
	UpdateAccounts(ctx context.Context, batch []external.Account) error
	GetLatestTime(ctx context.Context, entity interface{}, tenant interface{}) (time.Time, error)
	Insert(ctx context.Context, batch interface{}) error
}

type Executor interface {
	Name() string
	Run(ctx context.Context, sync *Sync) (int, error)
}

type Sync struct {
	*Context
	*external.Client
	Now    time.Time
	report *output.Leveled
}

func (s *Sync) Run(ctx context.Context, executor Executor) error {
	name := executor.Name()
	count, err := executor.Run(ctx, s)
	if err != nil {
		if flu.IsContextRelated(err) {
			return err
		} else {
			s.report.Errorf(name, err.Error())
		}
	} else {
		s.report.Infof("%s\n%d items synced", name, count)
	}

	return nil
}
