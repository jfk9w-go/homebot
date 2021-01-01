package statement

import (
	"log"

	"github.com/pkg/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type Data interface {
	OnUpdate() []clause.Expression
	Cancel() interface{}
}

type Iterator func(interface{}) error

type Iterable interface {
	Data
	ForEach(Iterator) error
}

type Database struct {
	*gorm.DB
}

func NewDatabase(datasource string, tables ...interface{}) (*Database, error) {
	dialector := postgres.Open(datasource)
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, errors.Wrap(err, "open database")
	}

	if err := db.AutoMigrate(tables...); err != nil {
		return nil, errors.Wrap(err, "migrate database")
	}

	return &Database{DB: db}, nil
}

func (db *Database) Update(iterable Iterable) error {
	cancel := iterable.Cancel()
	count := 0
	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		tx = tx.Clauses(iterable.OnUpdate()...)
		return iterable.ForEach(func(value interface{}) error {
			count++
			if cancel != nil && value == cancel {
				return errors.New("cancel")
			}

			return tx.Create(value).Error
		})
	}); err != nil {
		return err
	}

	log.Printf("Updated %d records for %T", count, cancel)
	return nil
}

func (db *Database) Close() error {
	_db, err := db.DB.DB()
	if err != nil {
		return errors.Wrap(err, "DB()")
	}

	return _db.Close()
}
