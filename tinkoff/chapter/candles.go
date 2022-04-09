package chapter

import (
	"context"
	"homebot/common"
	"homebot/tinkoff"
	"homebot/tinkoff/external"
	"time"

	"github.com/jfk9w-go/flu/syncf"

	"github.com/jfk9w-go/flu/apfel"
	"github.com/jfk9w-go/flu/gormf"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type Candles[C tinkoff.Context] struct {
	clock   syncf.Clock
	storage tinkoff.Storage[C]
}

func (m *Candles[C]) Title() string {
	return "Candles"
}

func (m *Candles[C]) String() string {
	return "tinkoff.chapter.candles"
}

func (m *Candles[C]) Include(ctx context.Context, app apfel.MixinApp[C]) error {
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

func (m *Candles[C]) Sync(ctx context.Context, client *external.Client, period time.Duration) ([]tinkoff.Chapter, int, error) {
	now := m.clock.Now()
	startTime := now.Add(-period)
	positions, err := m.storage.GetTradingPositions(ctx, startTime, client.Username())
	if err != nil {
		return nil, 0, errors.Wrap(err, "load trading positions from db")
	}

	if len(positions) == 0 {
		return nil, 0, nil
	}

	for _, position := range positions {
		var (
			buyTime  = external.TradingOperationsStart
			sellTime = now
		)

		if position.BuyTime != nil {
			buyTime = common.TrimDate(*position.BuyTime)
		}

		if buyTime.Before(startTime) {
			buyTime = startTime
		}

		if position.SellTime != nil {
			sellTime = common.TrimDate(*position.SellTime)
		}

		candles, err := client.Candles(ctx, position.Ticker, "D", buyTime, sellTime)
		if err != nil {
			return nil, 0, errors.Wrapf(err, "get candles for %s [%s, %s]", position.Ticker, buyTime, sellTime)
		}

		if len(candles) == 0 {
			continue
		}

		if err := m.storage.Tx(ctx, func(tx *gorm.DB) error {
			return gormf.Batch[external.Candle](candles).Ensure(tx, "primaryKey")
		}); err != nil {
			return nil, 0, errors.Wrap(err, "update")
		}
	}

	return nil, len(positions), nil
}
