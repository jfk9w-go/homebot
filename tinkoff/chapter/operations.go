package chapter

import (
	"context"
	"homebot/tinkoff"
	"homebot/tinkoff/external"
	"time"

	"github.com/jfk9w-go/flu/gormf"
	"github.com/jfk9w-go/flu/syncf"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type Operations[C tinkoff.Context] struct {
	receipts bool
	account  external.Account
	clock    syncf.Clock
	storage  tinkoff.Storage[C]
}

func (o *Operations[C]) Title() string {
	return o.account.Name
}

func (o *Operations[C]) String() string {
	return "tinkoff.chapter.operations." + o.account.ID
}

func (o *Operations[C]) Sync(ctx context.Context, client *external.Client, period time.Duration) ([]tinkoff.Chapter, int, error) {
	now := o.clock.Now()
	since, err := o.storage.GetLatestTime(ctx, new(external.Operation), o.account.ID)
	if err != nil {
		return nil, 0, errors.Wrap(err, "calculate last operation date from db")
	}

	if since.UnixNano() > 0 {
		since = since.Add(-period)
	}

	operations, err := client.Operations(ctx, now, o.account.ID, since)
	if err != nil {
		return nil, 0, errors.Wrap(err, "load operations from tinkoff")
	}

	if o.receipts {
		for _, operation := range operations {
			if operation.HasShoppingReceipt {
				if receipt, err := client.ShoppingReceipt(ctx, operation.ID); err != nil {
					return nil, 0, errors.Wrapf(err, "get receipt for #%d", operation.ID)
				} else {
					operation.ShoppingReceipt = receipt
				}
			}
		}
	}

	if len(operations) == 0 {
		return nil, 0, nil
	}

	if err := o.storage.Tx(ctx, func(tx *gorm.DB) error {
		tx, err := gormf.Filter(tx, operations, "tenant", "=", o.account.ID)
		if err != nil {
			return err
		}

		return gormf.Batch[external.Operation](operations).EnsureSince(tx, since, "primaryKey")
	}); err != nil {
		return nil, 0, errors.Wrap(err, "update")
	}

	return nil, len(operations), nil
}
