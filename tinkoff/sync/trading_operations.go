package sync

import (
	"context"

	"homebot/tinkoff"
	"homebot/tinkoff/external"

	"github.com/pkg/errors"
)

var TradingOperations tinkoff.Executor = tradingOperations{}

type tradingOperations struct{}

func (tradingOperations) Name() string {
	return "Trading operations"
}

func (tradingOperations) Run(ctx context.Context, sync *tinkoff.Sync) (int, error) {
	since, err := sync.GetLatestTime(ctx, new(external.TradingOperation), sync.Username())
	if err != nil {
		return 0, errors.Wrap(err, "get latest time)")
	}

	if since.UnixNano() > 0 {
		since = since.Add(-sync.Reload)
	}

	operations, err := sync.TradingOperations(ctx, sync.Now, since)
	if err != nil {
		return 0, errors.Wrap(err, "get")
	}

	if len(operations) == 0 {
		return 0, nil
	}

	if err := sync.Insert(ctx, operations); err != nil {
		return 0, errors.Wrap(err, "update")
	}

	return len(operations), nil
}
