package tinkoff

import (
	"context"
	"time"

	"homebot/tinkoff/external"

	"github.com/jfk9w-go/flu"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/html"
)

var CredentialsCodec = flu.Gob

type CredentialStore map[telegram.ID]external.Credential

func (creds CredentialStore) EncodeTo(output flu.Output) error {
	return flu.EncodeTo(CredentialsCodec(creds), output)
}

func DecodeCredentialsFrom(input flu.Input) (CredentialStore, error) {
	creds := make(CredentialStore)
	return creds, flu.DecodeFrom(input, CredentialsCodec(&creds))
}

type TradingPosition struct {
	Ticker   string
	BuyTime  *time.Time
	SellTime *time.Time
}

type Storage interface {
	UpdateAccounts(ctx context.Context, batch []external.Account) error
	GetLatestTime(ctx context.Context, entity interface{}, tenant interface{}) (time.Time, error)
	GetTradingPositions(ctx context.Context, from time.Time, username string) ([]TradingPosition, error)
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
	Reload time.Duration
	report *html.Writer
}

func (s *Sync) Run(ctx context.Context, executor Executor) error {
	count, err := executor.Run(ctx, s)
	s.report.Bold("\n%s • ", executor.Name())
	if err != nil {
		if flu.IsContextRelated(err) {
			return err
		} else {
			s.report.Text(err.Error())
		}
	} else {
		s.report.Text("%d items synced", count)
	}

	return nil
}

func (s *Sync) Close() error {
	return s.report.Flush()
}
