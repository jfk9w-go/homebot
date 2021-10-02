package tinkoff

import (
	"context"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/homebot/core"
	"github.com/jfk9w-go/homebot/ext/tinkoff/external"
	telegram "github.com/jfk9w-go/telegram-bot-api"
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
	report *core.JobReport
}

func (s *Sync) Run(ctx context.Context, executor Executor) error {
	name := executor.Name()
	count, err := executor.Run(ctx, s)
	if err != nil {
		if flu.IsContextRelated(err) {
			return err
		} else {
			s.report.Error(name, err.Error())
		}
	} else {
		s.report.Success(name, "%d", count)
	}

	return nil
}
