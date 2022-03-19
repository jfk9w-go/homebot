package sync

import (
	"context"

	"homebot/tinkoff"
	"homebot/tinkoff/external"

	"github.com/pkg/errors"
)

type Operations struct {
	external.Account
	Receipts bool
}

func (o Operations) Name() string {
	return o.Account.Name
}

func (o Operations) Run(ctx context.Context, sync *tinkoff.Sync) (int, error) {
	since, err := sync.GetLatestTime(ctx, new(external.Operation), o.Account.ID)
	if err != nil {
		return 0, errors.Wrap(err, "get latest time")
	}

	if since.UnixNano() > 0 {
		since = since.Add(-sync.Reload)
	}

	operations, err := sync.Operations(ctx, sync.Now, o.Account.ID, since)
	if err != nil {
		return 0, errors.Wrap(err, "get")
	}

	if o.Receipts {
		for _, operation := range operations {
			if operation.HasShoppingReceipt {
				if receipt, err := sync.ShoppingReceipt(ctx, operation.ID); err != nil {
					sync.Report.Bold("\nShopping receipt %d • ", operation.ID).Text("%v", err)
				} else {
					operation.ShoppingReceipt = receipt
				}
			}
		}
	}

	if len(operations) == 0 {
		return 0, nil
	}

	if err := sync.Insert(ctx, operations); err != nil {
		return 0, errors.Wrap(err, "update")
	}

	return len(operations), nil
}
