package tinkoff

import (
	"context"

	"github.com/pkg/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Database struct {
	*gorm.DB
}

func NewDatabase(datasource string, tables ...interface{}) (*Database, error) {
	dialector := postgres.Open(datasource)
	db, err := gorm.Open(dialector, &gorm.Config{ /*Logger: logger.Default.LogMode(logger.Silent)*/ })
	if err != nil {
		return nil, errors.Wrap(err, "open database")
	}

	if err := db.AutoMigrate(tables...); err != nil {
		return nil, errors.Wrap(err, "migrate database")
	}

	db.FullSaveAssociations = true
	return &Database{DB: db}, nil
}

func (db *Database) Update(ctx context.Context, data interface{}) error {
	return db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).
		CreateInBatches(data, 1).
		Error
}

func (db *Database) Close() error {
	_db, err := db.DB.DB()
	if err != nil {
		return errors.Wrap(err, "DB()")
	}

	return _db.Close()
}
