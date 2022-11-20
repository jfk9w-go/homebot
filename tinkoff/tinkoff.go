package tinkoff

import (
	"context"
	"time"

	"homebot/3rdparty/tinkoff"

	"gorm.io/gorm"
)

type (
	Credential  = tinkoff.Credential
	ConfirmFunc = tinkoff.ConfirmFunc
)

type Client interface {
	String() string
	GetAccounts(ctx context.Context, req tinkoff.Accounts) ([]tinkoff.Account, error)
	GetOperations(ctx context.Context, req tinkoff.Operations) ([]tinkoff.Operation, error)
	GetShoppingReceipt(ctx context.Context, req tinkoff.OperationReceipt) (*tinkoff.ShoppingReceipt, error)
	GetTradingOperations(ctx context.Context, req tinkoff.TradingOperations) ([]tinkoff.TradingOperation, error)
	GetPurchasedSecurities(ctx context.Context, req tinkoff.PurchasedSecurities) ([]tinkoff.PurchasedSecurity, error)
	GetCandles(ctx context.Context, req tinkoff.Candles) ([]tinkoff.Candle, error)
}

type TradingPosition struct {
	Ticker   string
	BuyTime  *time.Time
	SellTime *time.Time
}

type StorageInterface interface {
	RefreshAccounts(ctx context.Context, username string, accounts []tinkoff.Account) error
	GetOperationRefreshIntervalStart(ctx context.Context, accountID string) (time.Time, error)
	RefreshOperations(ctx context.Context, accountID string, since time.Time, operations []tinkoff.Operation) error
	GetPendingShoppingReceiptOperationIDs(ctx context.Context, accountID string) ([]uint64, error)
	StoreShoppingReceipt(ctx context.Context, receipt *tinkoff.ShoppingReceipt) error
	RemoveShoppingReceiptFlag(ctx context.Context, operationID uint64) error
	GetLatestTime(ctx context.Context, entity interface{}, tenant interface{}) (latestTime time.Time, err error)
	GetTradingPositions(ctx context.Context, from time.Time, username string) ([]TradingPosition, error)
	DB(ctx context.Context) *gorm.DB
	Tx(ctx context.Context, tx func(tx *gorm.DB) error) error
}
