package common

import (
	"context"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm/schema"

	"github.com/pkg/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type DB gorm.DB

func NewDB(datasource string, tables ...interface{}) (*DB, error) {
	dialector := postgres.Open(datasource)
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Error)})
	if err != nil {
		return nil, errors.Wrap(err, "open database")
	}

	if err := db.AutoMigrate(tables...); err != nil {
		return nil, errors.Wrap(err, "migrate database")
	}

	db.FullSaveAssociations = true
	return (*DB)(db), nil
}

func (db *DB) Unmask() *gorm.DB {
	return (*gorm.DB)(db)
}

func (db *DB) Exists(ctx context.Context, entity interface{}, tenant ...interface{}) (bool, error) {
	tx := db.Unmask().WithContext(ctx)
	tx = db.addTenantFilter(ctx, tx, entity, tenant)

	var count int64
	if err := tx.Model(entity).Limit(1).Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (db *DB) Delete(ctx context.Context, entity interface{}, since time.Time, tenant ...interface{}) error {
	tx := db.Unmask().WithContext(ctx)
	tx = db.addTenantFilter(ctx, tx, entity, tenant)
	return tx.Where("time >= ?", since).Delete(entity).Error
}

var UpdateInterval = 60 * 24 * time.Hour

func (db *DB) UpdateSince(ctx context.Context, entity interface{}, tenant ...interface{}) (since time.Time, err error) {
	var exists bool
	if exists, err = db.Exists(ctx, entity, tenant); err == nil && exists {
		if exists {
			now := ctx.Value("now").(time.Time)
			since = now.Add(-UpdateInterval)
			err = db.Delete(ctx, entity, since, tenant...)
		}
	}

	return
}

func (db *DB) Update(ctx context.Context, data interface{}) error {
	tx := db.Unmask().WithContext(ctx)
	tx = addOnConflictClause(tx, data)
	return tx.CreateInBatches(data, 100).Error
}

func (db *DB) Close() error {
	instance, err := db.Unmask().DB()
	if err != nil {
		return errors.Wrap(err, "DB()")
	}

	return instance.Close()
}

var namingStrategy schema.NamingStrategy

func addOnConflictClause(tx *gorm.DB, entity interface{}) *gorm.DB {
	taggedColumns := collectTaggedColumns(entity, "primaryKey")
	if len(taggedColumns) == 0 {
		return tx
	}

	columns := make([]clause.Column, len(taggedColumns))
	for i, column := range taggedColumns {
		columns[i] = clause.Column{Name: column}
	}

	return tx.Clauses(clause.OnConflict{
		Columns:   columns,
		UpdateAll: true,
	})
}

func (db *DB) addTenantFilter(ctx context.Context, tx *gorm.DB, entity interface{}, values ...interface{}) *gorm.DB {
	columns := collectTaggedColumns(entity, "tenant")
	if len(columns) != len(values) {
		db.Unmask().Logger.Error(ctx, "Tenant values [%v] size is not equal to tenant columns [%v] size", values, columns)
		return tx
	}

	for i, column := range columns {
		tx = tx.Where(column+" = ?", values[i])
	}

	return tx
}

func collectTaggedColumns(entity interface{}, setting string) []string {
	setting = strings.ToUpper(setting)
	entityType := reflect.TypeOf(entity).Elem()
	taggedColumns := make([]string, 0)
	for i := 0; i < entityType.NumField(); i++ {
		field := entityType.Field(i)
		if tag, ok := field.Tag.Lookup("gorm"); ok {
			tagSettings := schema.ParseTagSetting(tag, ";")
			if _, ok := tagSettings[setting]; !ok {
				continue
			}

			columnName, ok := tagSettings["COLUMN"]
			if !ok {
				columnName = namingStrategy.ColumnName("", field.Name)
			}

			taggedColumns = append(taggedColumns, columnName)
		}
	}

	return taggedColumns
}
