package sync

import (
	"context"

	"homebot/tinkoff"
	"homebot/tinkoff/external"

	"github.com/pkg/errors"
)

var Accounts tinkoff.Executor = accounts{}

type accounts struct{}

func (accounts) Name() string {
	return "Accounts"
}

func (accounts) Run(ctx context.Context, sync *tinkoff.Sync) (int, error) {
	accounts, err := sync.Accounts(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "get accounts")
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
		return 0, nil
	}

	if err := sync.UpdateAccounts(ctx, importantAccounts); err != nil {
		return 0, errors.Wrap(err, "update")
	} else {
		for _, account := range importantAccounts {
			if err := sync.Run(ctx, Operations{account}); err != nil {
				return 0, err
			}
		}
	}

	return len(importantAccounts), nil
}