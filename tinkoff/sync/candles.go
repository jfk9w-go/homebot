package sync

import (
	"context"
	"homebot/common"
	"homebot/tinkoff"
	"homebot/tinkoff/external"

	"github.com/pkg/errors"
)

var Candles tinkoff.Executor = candles{}

type candles struct{}

func (c candles) Name() string {
	return "Candles"
}

func (c candles) Run(ctx context.Context, sync *tinkoff.Sync) (int, error) {
	startTime := sync.Now.Add(-sync.Reload)
	positions, err := sync.GetTradingPositions(ctx, startTime, sync.Username())
	if err != nil {
		return 0, errors.Wrap(err, "get trading positions")
	}

	if len(positions) == 0 {
		return 0, nil
	}

	for _, position := range positions {
		var (
			buyTime  = external.TradingOperationsStart
			sellTime = sync.Now
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

		candles, err := sync.Candles(ctx, position.Ticker, "D", buyTime, sellTime)
		if err != nil {
			return 0, errors.Wrapf(err, "get candles for %s [%s, %s]", position.Ticker, buyTime, sellTime)
		}

		if len(candles) == 0 {
			continue
		}

		if err := sync.Insert(ctx, candles); err != nil {
			return 0, errors.Wrap(err, "update")
		}
	}

	return len(positions), nil
}
