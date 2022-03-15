package sync

import (
	"context"
	"strings"

	"github.com/jfk9w-go/flu/backoff"

	"homebot/tinkoff"
	"homebot/tinkoff/external"

	"github.com/pkg/errors"
)

type Operations struct {
	external.Account
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

	for i, operation := range operations {
		if operation.HasShoppingReceipt {
			if err := (backoff.Retry{
				Body: func(ctx context.Context) error {
					receipt, err := sync.ShoppingReceipt(ctx, operation.ID)
					switch {
					case err == nil:
						operations[i].ShoppingReceipt = receipt
						return nil
					case strings.Contains(err.Error(), "REQUEST_RATE_LIMIT_EXCEEDED"):
						return err
					default:
						sync.Report.Bold("\nReceipt %d • ", operation.ID).Text(err.Error())
						return nil
					}
				},
				Retries: 3,
				Backoff: backoff.Exp{Base: 2, Power: 1},
			}).Do(ctx); err != nil {
				return 0, errors.Wrapf(err, "get receipt %d", operation.ID)
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
