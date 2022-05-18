package tinkoff

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	"homebot/tinkoff/external"

	"github.com/jfk9w-go/flu/apfel"
	"github.com/jfk9w-go/flu/gormf"
	"github.com/jfk9w-go/flu/logf"
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
	if !app.Config().TinkoffConfig().Enabled {
		return apfel.ErrDisabled
	}

	gorm := &apfel.GormDB[C]{Config: app.Config().TinkoffConfig().DB}
	if err := app.Use(ctx, gorm, false); err != nil {
		return errors.Wrap(err, "use gorm db")
	}

	db := gorm.DB()
	db.FullSaveAssociations = true
	if err := db.WithContext(ctx).AutoMigrate(
		external.Account{},
		external.Operation{},
		//external.OperationLocation{},
		external.OperationLoyaltyBonus{},
		external.ShoppingReceipt{},
		external.ShoppingReceiptItem{},
		external.TradingOperation{},
		external.PurchasedSecurity{},
		external.Candle{},
	); err != nil {
		return errors.Wrap(err, "auto migrate")
	}

	if err := db.WithContext(ctx).Exec(debitDDL).Error; err != nil {
		logf.Get(m).Errorf(ctx, "failed to create debit view: %+v", err)
	}

	if err := db.WithContext(ctx).Exec(creditDDL).Error; err != nil {
		logf.Get(m).Errorf(ctx, "failed to create credit view: %+v", err)
	}

	if err := db.WithContext(ctx).Exec(tradingPositionsDDL).Error; err != nil {
		logf.Get(m).Errorf(ctx, "failed to create trading positions view: %+v", err)
	}

	m.db = db
	return nil
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

type TradingPosition struct {
	Ticker   string
	BuyTime  *time.Time
	SellTime *time.Time
}

func (m *Storage[C]) GetTradingPositions(ctx context.Context, from time.Time, username string) ([]TradingPosition, error) {
	ps := make([]TradingPosition, 0)
	return ps, m.db.WithContext(ctx).
		Table("trading_positions").
		Where("(sell_time is null or sell_time >= ?) and username = ?", from, username).
		Scan(&ps).
		Error
}

func (m *Storage[C]) Tx(ctx context.Context, tx func(db *gorm.DB) error) error {
	return m.db.WithContext(ctx).Transaction(tx)
}
