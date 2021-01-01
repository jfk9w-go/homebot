package statement

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/csv"
	"github.com/pkg/errors"
	"gorm.io/gorm/clause"
)

type Status string

const (
	OK     Status = "OK"
	Failed Status = "FAILED"
)

func Currency(value string) string {
	value = strings.ToUpper(value)
	if value == "RUR" {
		return "RUB"
	}

	return value
}

type BankStatement struct {
	Bank            string     `gorm:"column:bank;not null"`
	Username        string     `gorm:"column:username;not null"`
	AccountName     string     `gorm:"column:account_name;not null"`
	Account         string     `gorm:"column:account;not null"`
	Moment          time.Time  `gorm:"column:moment;type:timestamp;not null"`
	PaymentDay      *time.Time `gorm:"column:payment_day;type:date"`
	CardNumber      string     `gorm:"column:card_number"`
	Status          Status     `gorm:"column:status;not null"`
	Amount          float64    `gorm:"column:amount;not null"`
	Currency        string     `gorm:"column:currency;type:varchar(3);not null"`
	PaymentAmount   float64    `gorm:"column:payment_amount;not null"`
	PaymentCurrency string     `gorm:"column:payment_currency;type:varchar(3);not null"`
	Cashback        float64    `gorm:"column:cashback;not null"`
	Category        string     `gorm:"column:category"`
	MCC             string     `gorm:"column:mcc;type:varchar(5)"`
	Description     string     `gorm:"column:description;not null"`
	Bonus           float64    `gorm:"column:bonus;not null"`
}

func (*BankStatement) TableName() string {
	return "bank_statement"
}

type BankStatementParser interface {
	Parse(source *csv.Row, target *BankStatement) error
}

type BankStatementIterable chan *BankStatement

func (i BankStatementIterable) OnUpdate() []clause.Expression {
	return []clause.Expression{
		clause.OnConflict{
			DoNothing: true,
		},
	}
}

func (i BankStatementIterable) Cancel() interface{} {
	return bankStatementCancel
}

func (i BankStatementIterable) ForEach(iterator Iterator) error {
	for st := range i {
		if err := iterator(&st); err != nil {
			return err
		}
	}

	return nil
}

var bankStatementCancel = new(BankStatement)

type BatchOutput interface {
	Update(Iterable) error
}

type JSONBatchOutput struct {
	io.Writer
}

func NewJSONOutput(output flu.Output) (JSONBatchOutput, error) {
	writer, err := output.Writer()
	return JSONBatchOutput{writer}, err
}

func (o JSONBatchOutput) Update(iterable Iterable) error {
	cancel := iterable.Cancel()
	return iterable.ForEach(func(value interface{}) error {
		if value == cancel {
			return errors.New("canceled")
		}

		data, err := json.Marshal(value)
		if err != nil {
			return errors.Wrap(err, "marshal")
		}

		if _, err := o.Write(data); err != nil {
			return errors.Wrap(err, "write json")
		}

		if _, err := o.Write([]byte{'\n'}); err != nil {
			return errors.Wrap(err, "write new line")
		}

		return nil
	})
}

func (o JSONBatchOutput) Close() error {
	return flu.Close(o.Writer)
}
