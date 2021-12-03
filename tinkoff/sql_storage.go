package tinkoff

import (
	"context"
	"fmt"
	"time"

	"homebot/tinkoff/external"

	"github.com/jfk9w-go/flu/gormf"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type SQLStorage gorm.DB

func (s *SQLStorage) Unmask() *gorm.DB {
	return (*gorm.DB)(s)
}

func (s *SQLStorage) Init(ctx context.Context) error {
	db := s.Unmask()
	db.FullSaveAssociations = true
	return db.WithContext(ctx).AutoMigrate(
		external.Account{},
		external.Operation{},
		//OperationLocation{},
		external.OperationLoyaltyBonus{},
		external.ShoppingReceipt{},
		external.ShoppingReceiptItem{},
		external.TradingOperation{},
		external.PurchasedSecurity{},
	)
}

func (s *SQLStorage) UpdateAccounts(ctx context.Context, accounts []external.Account) error {
	return s.Unmask().WithContext(ctx).
		Clauses(gormf.OnConflictClause(accounts, "primaryKey", true, nil)).
		Create(accounts).
		Error
}

func (s *SQLStorage) GetLatestTime(ctx context.Context, entity interface{}, tenant interface{}) (time.Time, error) {
	timeColumns := gormf.CollectTaggedColumns(entity, "time")
	if len(timeColumns) == 0 {
		return time.Time{}, errors.Errorf("no primary keys in %T", entity)
	}

	timeColumn := timeColumns[0]
	tx := s.Unmask().WithContext(ctx)
	tx, err := addTenantFilter(tx, entity, tenant)
	if err != nil {
		return time.Time{}, err
	}

	latestTime := new(*time.Time)
	if err := tx.Model(entity).
		Select(fmt.Sprintf(`max("%s")`, timeColumn)).
		Scan(latestTime).
		Error; err != nil {
		return time.Time{}, err
	}

	if *latestTime == nil {
		return time.Time{}, nil
	}

	return **latestTime, nil
}

func (s *SQLStorage) Insert(ctx context.Context, batch interface{}) error {
	return s.Unmask().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(batch).Error; err != nil {
			return err
		}

		return tx.CreateInBatches(batch, 100).Error
	})
}

func addTenantFilter(tx *gorm.DB, entity interface{}, values ...interface{}) (*gorm.DB, error) {
	columns := gormf.CollectTaggedColumns(entity, "tenant")
	if len(columns) != len(values) {
		return nil, errors.Errorf("tenant values [%v] size is not equal to tenant columns [%v] size", values, columns)
	}

	for i, column := range columns {
		tx = tx.Where(column+" = ?", values[i])
	}

	return tx, nil
}
