package tinkoff

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"homebot/3rdparty/tinkoff"

	"gorm.io/gorm"

	"github.com/jfk9w-go/flu/gormf"
	"github.com/jfk9w-go/flu/syncf"
	"github.com/pkg/errors"
)

type canvas struct {
	Client
	StorageInterface
	logger
	overlap  time.Duration
	username string
}

type chapter interface {
	name() string
	sync(ctx context.Context, cvs *canvas) ([]chapter, error)
}

type accountsChapter struct{}

func (accountsChapter) name() string {
	return "🏦 Accounts"
}

func (accountsChapter) sync(ctx context.Context, cvs *canvas) ([]chapter, error) {
	accounts, err := cvs.GetAccounts(ctx, tinkoff.Accounts{})
	if err != nil {
		return nil, errors.Wrap(err, "retrieve")
	}

	if err := cvs.RefreshAccounts(ctx, accounts); err != nil {
		cvs.warnf(ctx, "refresh %d accounts in db: %v", len(accounts), err)
	} else if len(accounts) == 0 {
		return nil, nil
	}

	cvs.infof(ctx, "%d accounts updated", len(accounts))

	chapters := make([]chapter, 2*len(accounts))
	suspended := new(atomic.Value)
	for i, account := range accounts {
		chapters[i] = operationsChapter{
			account: account,
		}

		chapters[len(accounts)+i] = shoppingReceiptsChapter{
			account:   account,
			suspended: suspended,
		}
	}

	return chapters, nil
}

type operationsChapter struct {
	account tinkoff.Account
}

func (c operationsChapter) name() string {
	return fmt.Sprintf("💳 %s", c.account)
}

func (c operationsChapter) sync(ctx context.Context, cvs *canvas) ([]chapter, error) {
	refreshStart, err := cvs.GetOperationRefreshIntervalStart(ctx, c.account.ID)
	if err != nil {
		return nil, errors.Wrap(err, "get refresh interval start")
	}

	if !refreshStart.IsZero() {
		refreshStart = refreshStart.Add(-cvs.overlap)
	}

	operations, err := cvs.GetOperations(ctx, tinkoff.Operations{
		AccountID: c.account.ID,
		Start:     refreshStart,
	})

	if err != nil {
		return nil, errors.Wrap(err, "retrieve operations")
	}

	if err := cvs.RefreshOperations(ctx, c.account.ID, refreshStart, operations); err != nil {
		return nil, errors.Wrapf(err, "refresh %d operations in db", len(operations))
	}

	cvs.infof(ctx, "%d operations updated since %s", len(operations), refreshStart)

	return nil, nil
}

type shoppingReceiptsChapter struct {
	account   tinkoff.Account
	suspended *atomic.Value
}

func (c shoppingReceiptsChapter) name() string {
	return fmt.Sprintf("💳 %s", c.account)
}

func (c shoppingReceiptsChapter) sync(ctx context.Context, cvs *canvas) ([]chapter, error) {
	if suspended, _ := c.suspended.Load().(bool); suspended {
		return nil, errors.New("receipt sync is suspended now, try again later")
	}

	pendingReceiptIDs, err := cvs.GetPendingShoppingReceiptOperationIDs(ctx, c.account.ID)
	if err != nil {
		return nil, errors.Wrap(err, "get pending shopping receipt operation ids")
	}

	for i, operationID := range pendingReceiptIDs {
		receipt, err := cvs.GetShoppingReceipt(ctx, tinkoff.OperationReceipt{
			OperationID: operationID,
		})

		switch {
		case errors.Is(err, tinkoff.ErrRequestRateLimitExceeded) || syncf.IsContextRelated(err):
			c.suspended.Store(true)
			return nil, errors.Wrapf(err, "retrieve receipt %d/%d", i+1, len(pendingReceiptIDs))
		case errors.Is(err, tinkoff.ErrNoDataFound):
			if err := cvs.RemoveShoppingReceiptFlag(ctx, operationID); err != nil {
				cvs.warnf(ctx, "remove receipt flag %d/%d: %v", i+1, len(pendingReceiptIDs), err)
			}

			fallthrough
		case err != nil:
			continue
		}

		if err := cvs.StoreShoppingReceipt(ctx, receipt); err != nil {
			cvs.warnf(ctx, "store receipt %d/%d: %v", i+1, len(pendingReceiptIDs), err)
		}
	}

	if len(pendingReceiptIDs) > 0 {
		cvs.infof(ctx, "%d receipts updated", len(pendingReceiptIDs))
	}

	return nil, nil
}

type tradingOperationsChapter struct{}

func (tradingOperationsChapter) name() string {
	return "💸 Trading operations"
}

func (tradingOperationsChapter) sync(ctx context.Context, cvs *canvas) ([]chapter, error) {
	latestTime, err := cvs.GetLatestTime(ctx, new(tinkoff.TradingOperation), cvs.username)
	if err != nil {
		return nil, errors.Wrap(err, "get latest time")
	}

	if !latestTime.IsZero() {
		latestTime = latestTime.Add(-cvs.overlap)
	}

	items, err := cvs.GetTradingOperations(ctx, tinkoff.TradingOperations{From: latestTime})
	if err != nil {
		return nil, errors.Wrap(err, "retrieve")
	}

	if err := cvs.Tx(ctx, func(tx *gorm.DB) error {
		tx, err := gormf.Filter(tx, items, "tenant", "=", cvs.username)
		if err != nil {
			return err
		}

		batch := gormf.Batch[tinkoff.TradingOperation](items)
		return batch.Ensure(tx, "primaryKey")
	}); err != nil {
		return nil, errors.Wrapf(err, "update %d operations in db", len(items))
	}

	if len(items) > 0 {
		cvs.infof(ctx, "%d operations updated since %s", len(items), latestTime)
	}

	return []chapter{
		purchasedSecuritiesChapter{},
		candlesChapter{},
	}, nil
}

type purchasedSecuritiesChapter struct{}

func (purchasedSecuritiesChapter) name() string {
	return "🔐 Purchased securities"
}

func (purchasedSecuritiesChapter) sync(ctx context.Context, cvs *canvas) ([]chapter, error) {
	items, err := cvs.GetPurchasedSecurities(ctx, tinkoff.TinkoffRUB)
	if err != nil {
		return nil, errors.Wrap(err, "retrieve")
	}

	if err := cvs.Tx(ctx, func(tx *gorm.DB) error {
		batch := gormf.Batch[tinkoff.PurchasedSecurity](items)
		return batch.Ensure(cvs.DB(ctx), "primaryKey")
	}); err != nil {
		return nil, errors.Wrapf(err, "store %d items in db", len(items))
	}

	if len(items) > 0 {
		cvs.infof(ctx, "%d items updated", len(items))
	}

	return nil, nil
}

type candlesChapter struct{}

func (candlesChapter) name() string {
	return "🕯 Candles"
}

func (candlesChapter) sync(ctx context.Context, cvs *canvas) ([]chapter, error) {
	return nil, nil
}
