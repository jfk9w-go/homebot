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

type PurchasedSecurities[C tinkoff.Context] struct {
	clock   syncf.Clock
	storage tinkoff.Storage[C]
}

func (m *PurchasedSecurities[C]) Title() string {
	return "Purchased securities"
}

func (m *PurchasedSecurities[C]) String() string {
	return "tinkoff.chapter.purchased-securities"
}

func (m *PurchasedSecurities[C]) Include(ctx context.Context, app apfel.MixinApp[C]) error {
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

func (m *PurchasedSecurities[C]) Sync(ctx context.Context, client *external.Client, period time.Duration) ([]tinkoff.Chapter, int, error) {
	now := m.clock.Now()
	securities, err := client.PurchasedSecurities(ctx, now)
	if err != nil {
		return nil, 0, errors.Wrap(err, "load purchased securities from tinkoff")
	}

	if err := m.storage.Tx(ctx, func(db *gorm.DB) error {
		return gormf.Batch[external.PurchasedSecurity](securities).Ensure(db, "primaryKey")
	}); err != nil {
		return nil, 0, errors.Wrap(err, "update purchased securities in db")
	}

	return nil, len(securities), nil
}
