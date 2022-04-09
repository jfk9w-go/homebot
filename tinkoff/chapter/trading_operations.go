package chapter

import (
	"context"
	"homebot/tinkoff"
	"homebot/tinkoff/external"
	"time"

	"github.com/jfk9w-go/flu/syncf"

	"github.com/jfk9w-go/flu/apfel"
	"github.com/jfk9w-go/flu/gormf"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type TradingOperations[C tinkoff.Context] struct {
	clock   syncf.Clock
	storage tinkoff.Storage[C]
}

func (m *TradingOperations[C]) Title() string {
	return "Trading operations"
}

func (m *TradingOperations[C]) String() string {
	return "tinkoff.chapter.trading-operations"
}

func (m *TradingOperations[C]) Include(ctx context.Context, app apfel.MixinApp[C]) error {
	if !app.Config().TinkoffConfig().Chapters[m.String()] {
		return apfel.ErrDisabled
	}

	var storage *tinkoff.Storage[C] = &m.storage
	if err := app.Use(ctx, storage, false); err != nil {
		return errors.Wrap(err, "use storage")
	}

	m.clock = app
	return nil
}

func (m *TradingOperations[C]) Sync(ctx context.Context, client *external.Client, period time.Duration) ([]tinkoff.Chapter, int, error) {
	now := m.clock.Now()
	since, err := m.storage.GetLatestTime(ctx, new(external.TradingOperation), client.Username())
	if err != nil {
		return nil, 0, errors.Wrap(err, "calculate latest trading operation time from db")
	}

	if since.UnixNano() > 0 {
		since = since.Add(-period)
	}

	operations, err := client.TradingOperations(ctx, now, since)
	if err != nil {
		return nil, 0, errors.Wrap(err, "load trading operations from tinkoff")
	}

	if len(operations) == 0 {
		return nil, 0, nil
	}

	if err := m.storage.Tx(ctx, func(tx *gorm.DB) error {
		tx, err := gormf.Filter(tx, operations, "tenant", "=", client.Username())
		if err != nil {
			return err
		}

		return gormf.Batch[external.TradingOperation](operations).Ensure(tx, "primaryKey")
	}); err != nil {
		return nil, 0, errors.Wrap(err, "update trading operations in db")
	}

	return nil, len(operations), nil
}
