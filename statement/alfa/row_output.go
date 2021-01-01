package alfa

import (
	"context"
	"time"

	"github.com/jfk9w-go/bank-statement/statement"
	"github.com/jfk9w-go/flu/csv"
)

var (
	PaymentDayLayout = "02.01.06"
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
	Context  context.Context
	Out      chan<- *statement.BankStatement
	BankID   string
	Username string
}

func (o RowOutput) Output(source *csv.Row) error {
	target := new(statement.BankStatement)
	target.Bank = o.BankID
	target.Username = o.Username
	target.AccountName = source.String("Тип счёта")
	target.Account = source.String("Номер счета")
	target.Moment = source.Time("Дата операции", PaymentDayLayout, Location)
	target.PaymentDay = &target.Moment
	target.Status = statement.OK
	if source.String("Приход") != "0" {
		target.Amount = source.Float("Приход", 64)
	} else {
		target.Amount = -source.Float("Расход", 64)
	}
	target.Currency = statement.Currency(source.String("Валюта"))
	target.PaymentAmount = target.Amount
	target.PaymentCurrency = target.Currency
	target.Description = source.String("Референс проводки") + "; " + source.String("Описание операции")
	if source.Err != nil {
		return source.Err
	}

	select {
	case <-o.Context.Done():
		return o.Context.Err()
	case o.Out <- target:
		return nil
	}
}
