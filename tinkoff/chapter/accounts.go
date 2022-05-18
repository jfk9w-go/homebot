package chapter

import (
	"context"
	"time"

	"homebot/tinkoff"
	"homebot/tinkoff/external"

	"github.com/jfk9w-go/flu/apfel"
	"github.com/jfk9w-go/flu/gormf"
	"github.com/jfk9w-go/flu/syncf"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type Accounts[C tinkoff.Context] struct {
	receipts bool
	clock    syncf.Clock
	storage  tinkoff.Storage[C]
}

func (m *Accounts[C]) Title() string {
	return "Accounts"
}

func (m *Accounts[C]) String() string {
	return "tinkoff.chapter.accounts"
}

func (m *Accounts[C]) Include(ctx context.Context, app apfel.MixinApp[C]) error {
	if !app.Config().TinkoffConfig().Chapters[m.String()] {
		return apfel.ErrDisabled
	}

	var storage *tinkoff.Storage[C] = &m.storage
	if err := app.Use(ctx, storage, false); err != nil {
		return errors.Wrap(err, "inject storage")
	}

	m.clock = app
	m.receipts = app.Config().TinkoffConfig().Receipts
	return nil
}

func (m *Accounts[C]) Sync(ctx context.Context, client *external.Client, period time.Duration) ([]tinkoff.Chapter, int, error) {
	accounts, err := client.Accounts(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "load accounts from tinkoff")
	}

	importantAccounts := make([]external.Account, 0)
	for _, account := range accounts {
		if account.Type == "SharedCredit" ||
			account.Type == "SharedCurrent" ||
			account.Type == "ExternalAccount" {
			continue
		}

		importantAccounts = append(importantAccounts, account)
	}

	if len(importantAccounts) == 0 {
		return nil, 0, nil
	}

	if err := m.storage.Tx(ctx, func(tx *gorm.DB) error {
		return gormf.Batch[external.Account](importantAccounts).Ensure(tx, "primaryKey")
	}); err != nil {
		return nil, 0, errors.Wrap(err, "update accounts in db")
	}

	operations := make([]tinkoff.Chapter, len(importantAccounts))
	for i, account := range importantAccounts {
		operations[i] = &Operations[C]{
			storage:  m.storage,
			clock:    m.clock,
			account:  account,
			receipts: m.receipts,
		}
	}

	return operations, len(importantAccounts), nil
}
