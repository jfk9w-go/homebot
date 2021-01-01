package tinkoff

import (
	"context"
	"time"

	"github.com/jfk9w-go/bank-statement/statement"
	"github.com/jfk9w-go/flu/csv"
)

var (
	MomentLayout     = "02.01.2006 15:04:05"
	PaymentDayLayout = "02.01.2006"
	Location         *time.Location
)

func init() {
	var err error
	Location, err = time.LoadLocation("Europe/Moscow")
	if err != nil {
		panic(err)
	}
}

type RowOutput struct {
	Context     context.Context
	Out         chan<- *statement.BankStatement
	BankID      string
	Username    string
	AccountName string
	AccountID   string
}

func (o RowOutput) Output(source *csv.Row) error {
	target := new(statement.BankStatement)
	target.Bank = o.BankID
	target.Username = o.Username
	target.AccountName = o.AccountName
	target.Account = o.AccountID
	target.Moment = source.Time("Дата операции", MomentLayout, Location)
	if source.String("Дата платежа") != "" {
		value := source.Time("Дата платежа", PaymentDayLayout, Location)
		target.PaymentDay = &value
	}
	target.CardNumber = source.String("Номер карты")
	target.Status = statement.Status(source.String("Статус"))
	target.Amount = source.Float("Сумма операции", 64)
	target.Currency = statement.Currency(source.String("Валюта операции"))
	target.PaymentAmount = source.Float("Сумма платежа", 64)
	target.PaymentCurrency = source.String("Валюта платежа")
	if source.String("Кэшбэк") != "" {
		target.Cashback = source.Float("Кэшбэк", 64)
	}
	target.Category = source.String("Категория")
	target.MCC = source.String("MCC")
	target.Description = source.String("Описание")
	target.Bonus = source.Float("Бонусы (включая кэшбэк)", 64)
	if source.Err != nil {
		return source.Err
	}

	select {
	case o.Out <- target:
		return nil
	case <-o.Context.Done():
		return o.Context.Err()
	}
}
