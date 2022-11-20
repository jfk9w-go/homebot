package tinkoff

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	"homebot/3rdparty/tinkoff"

	"github.com/jfk9w-go/flu/apfel"
	"github.com/jfk9w-go/flu/gormf"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

var (
	//go:embed ddl/debit.sql
	debitDDL string

	//go:embed ddl/credit.sql
	creditDDL string

	//go:embed ddl/trading_positions.sql
	tradingPositionsDDL string
)

type Storage[C Context] struct {
	db *gorm.DB
}

func (m *Storage[C]) String() string {
	return "tinkoff.storage"
}

func (m *Storage[C]) Include(ctx context.Context, app apfel.MixinApp[C]) error {
	gorm := &apfel.GormDB[C]{Config: app.Config().TinkoffConfig().DB}
	if err := app.Use(ctx, gorm, false); err != nil {
		return err
	}

	db := gorm.DB()
	db.FullSaveAssociations = true
	if err := db.WithContext(ctx).AutoMigrate(
		tinkoff.Account{},
		tinkoff.Operation{},
		tinkoff.OperationLocation{},
		tinkoff.OperationLoyaltyBonus{},
		tinkoff.ShoppingReceipt{},
		tinkoff.ShoppingReceiptItem{},
		tinkoff.TradingOperation{},
		tinkoff.PurchasedSecurity{},
		tinkoff.Candle{},
	); err != nil {
		return errors.Wrap(err, "auto migrate")
	}

	if err := db.WithContext(ctx).Exec(debitDDL).Error; err != nil {
		return errors.Wrap(err, "create debit view")
	}

	if err := db.WithContext(ctx).Exec(creditDDL).Error; err != nil {
		return errors.Wrap(err, "create credit view")
	}

	if err := db.WithContext(ctx).Exec(tradingPositionsDDL).Error; err != nil {
		return errors.Wrap(err, "create trading_positions view")
	}

	m.db = db
	return nil
}

func (m *Storage[C]) RefreshAccounts(ctx context.Context, username string, accounts []tinkoff.Account) error {
	var model tinkoff.Account
	accountIDs := make([]string, len(accounts))
	for i, account := range accounts {
		accountIDs[i] = account.ID
	}

	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		onConflict := gormf.OnConflictClause(&model, "primaryKey", true, nil)
		if err := tx.
			Clauses(onConflict).
			CreateInBatches(accounts, 1000).
			Error; err != nil {
			return errors.Wrap(err, "create in batches")
		}

		if err := tx.
			Model(&model).
			Where("username = ? and id not in ?", username, accountIDs).
			Update("archived", true).
			Error; err != nil {
			return errors.Wrap(err, "archive old accounts")
		}

		return nil
	})
}

func (m *Storage[C]) GetOperationRefreshIntervalStart(ctx context.Context, accountID string) (time.Time, error) {
	var (
		model tinkoff.Operation
		value sql.NullTime
	)

	if err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model).
			Where("debiting_time is null and account_id = ?", accountID).
			Select("min(time)").
			Scan(&value).
			Error; err != nil {
			return errors.Wrap(err, "select min debiting time")
		}

		if value.Valid {
			return nil
		}

		if err := tx.Model(&model).
			Where("account_id = ?", accountID).
			Select("max(time)").
			Scan(&value).
			Error; err != nil {
			return errors.Wrap(err, "select max time")
		}

		return nil
	}); err != nil {
		return time.Time{}, err
	}

	return value.Time, nil
}

func (m *Storage[C]) RefreshOperations(ctx context.Context, accountID string, since time.Time, operations []tinkoff.Operation) error {
	var model tinkoff.Operation
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		deleteTx := tx.
			Where("debiting_time is null and account_id = ? and time >= ?", accountID, since).
			Delete(&model)
		if err := deleteTx.Error; err != nil {
			return errors.Wrap(err, "delete")
		}

		onConflict := gormf.OnConflictClause(&model, "primaryKey", true, nil)
		createTx := tx.Clauses(onConflict).
			CreateInBatches(operations, 1000)
		if err := createTx.Error; err != nil {
			return errors.Wrap(err, "create")
		}

		return nil
	})
}

func (m *Storage[C]) GetPendingShoppingReceiptOperationIDs(ctx context.Context, accountID string) ([]uint64, error) {
	var operationIDs []uint64
	if err := m.db.WithContext(ctx).Raw(`
		select distinct o.id 
		from operations o 
		left join shopping_receipts sr 
		on o.id = sr.operation_id
		where o.has_shopping_receipt
          and o.debiting_time is not null
          and sr.total_sum is null
          and o.account_id = ?`, accountID).
		Scan(&operationIDs).
		Error; err != nil {
		return nil, err
	}

	return operationIDs, nil
}

func (m *Storage[C]) StoreShoppingReceipt(ctx context.Context, receipt *tinkoff.ShoppingReceipt) error {
	return m.db.WithContext(ctx).Create(receipt).Error
}

func (m *Storage[C]) RemoveShoppingReceiptFlag(ctx context.Context, operationID uint64) error {
	return m.db.WithContext(ctx).
		Exec("update operations set has_shopping_receipt = false where id = ?", operationID).
		Error
}

func (m *Storage[C]) GetLatestTime(ctx context.Context, entity interface{}, tenant interface{}) (latestTime time.Time, err error) {
	timeColumns := gormf.CollectTaggedColumns(entity, "time")
	var timeColumn string
	for column := range timeColumns {
		timeColumn = column
	}

	if timeColumn == "" {
		err = errors.Errorf("no time column in %T", entity)
		return
	}

	tx := m.db.WithContext(ctx)
	tx, err = gormf.Filter(tx, entity, "tenant", "=", tenant)
	if err != nil {
		return
	}

	value := new(sql.NullTime)
	if err = tx.Model(entity).
		Select(fmt.Sprintf(`max("%s")`, timeColumn)).
		Scan(value).
		Error; err == nil && value.Valid {
		latestTime = value.Time
	}

	return
}

func (m *Storage[C]) DB(ctx context.Context) *gorm.DB {
	return m.db.WithContext(ctx)
}

func (m *Storage[C]) Tx(ctx context.Context, tx func(tx *gorm.DB) error) error {
	return m.db.WithContext(ctx).Transaction(tx)
}

func (m *Storage[C]) GetTradingPositions(ctx context.Context, from time.Time, username string) ([]TradingPosition, error) {
	ps := make([]TradingPosition, 0)
	return ps, m.db.WithContext(ctx).
		Table("trading_positions").
		Where("(sell_time is null or sell_time >= ?) and username = ?", from, username).
		Scan(&ps).
		Error
}
