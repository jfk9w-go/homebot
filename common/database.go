package common

import (
	"context"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type Database struct {
	*gorm.DB
}

func NewDatabase(datasource string, tables ...interface{}) (*Database, error) {
	dialector := postgres.Open(datasource)
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Error)})
	if err != nil {
		return nil, errors.Wrap(err, "open database")
	}

	if err := db.AutoMigrate(tables...); err != nil {
		return nil, errors.Wrap(err, "migrate database")
	}

	db.FullSaveAssociations = true
	return &Database{DB: db}, nil
}

func (db *Database) Exists(ctx context.Context, entity interface{}, tenant ...interface{}) (bool, error) {
	tx := db.WithContext(ctx)
	tx = whereByTenant(tx, entity, tenant)

	var count int64
	if err := tx.Model(entity).Limit(1).Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (db *Database) Delete(ctx context.Context, entity interface{}, since time.Time, tenant ...interface{}) error {
	tx := db.WithContext(ctx)
	tx = whereByTenant(tx, entity, tenant)

	return tx.Where("time >= ?", since).Delete(entity).Error
}

func (db *Database) Update(ctx context.Context, data interface{}) error {
	return db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).
		CreateInBatches(data, 100).
		Error
}

func (db *Database) Close() error {
	_db, err := db.DB.DB()
	if err != nil {
		return errors.Wrap(err, "DB()")
	}

	return _db.Close()
}

func whereByTenant(tx *gorm.DB, entity interface{}, tenant ...interface{}) *gorm.DB {
	entityType := reflect.TypeOf(entity).Elem()
	tenantIdx := 0
	for i := 0; i < entityType.NumField(); i++ {
		field := entityType.Field(i)
		if tenantColumn, ok := field.Tag.Lookup("tenant"); ok {
			tx = tx.Where(tenantColumn+" = ?", tenant[tenantIdx])
			tenantIdx++
		}
	}

	return tx
}
