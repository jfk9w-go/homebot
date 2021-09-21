package sync

import (
	"context"

	"github.com/pkg/errors"

	"github.com/jfk9w-go/homebot/ext/tinkoff"
)

var PurchasedSecurities tinkoff.Executor = purchasedSecurities{}

type purchasedSecurities struct{}

func (purchasedSecurities) Name() string {
	return "Purchased securities"
}

func (purchasedSecurities) Run(ctx context.Context, sync *tinkoff.Sync) (int, error) {
	securities, err := sync.PurchasedSecurities(ctx, sync.Now)
	if err != nil {
		return 0, errors.Wrap(err, "get")
	}

	if err := sync.Insert(ctx, securities); err != nil {
		return 0, errors.Wrap(err, "update")
	}

	return len(securities), nil
}
