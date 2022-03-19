package external

import (
	"strings"
	"time"
)

var (
	Host                        = "https://www.tinkoff.ru"
	LoginPage                   = Host + "/login/"
	PingEndpoint                = Host + "/api/common/v1/ping"
	SessionEndpoint             = Host + "/api/common/v1/session"
	SignUpEndpoint              = Host + "/api/common/v1/sign_up"
	ConfirmEndpoint             = Host + "/api/common/v1/confirm"
	LevelUpEndpoint             = Host + "/api/common/v1/level_up"
	GroupedRequestsEndpount     = Host + "/api/common/v1/grouped_requests"
	OperationsEndpoint          = Host + "/api/common/v1/operations"
	ShoppingReceiptEndpount     = Host + "/api/common/v1/shopping_receipt"
	TradingOperationsEndpoint   = Host + "/api/trading/user/operations"
	PurchasedSecuritiesEndpoint = Host + "/api/trading/portfolio/purchased_securities"
	CandlesEndpoint             = Host + "/api/trading/symbols/candles"

	TradingOperationsStart = time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)
	origin                 = "web,ib5,platform"
)

type ResultCodeError string

func (e ResultCodeError) Error() string {
	return string(e)
}

type Operations []Operation

func (os Operations) Len() int {
	return len(os)
}

func (os Operations) Less(i, j int) bool {
	return os[i].ID < os[j].ID
}

func (os Operations) Swap(i, j int) {
	os[i], os[j] = os[j], os[i]
}

type Errors []error

func (e Errors) Add(err error) Errors {
	return append(e, err)
}

func (e Errors) Error() string {
	b := new(strings.Builder)
	for i, err := range e {
		if i > 0 {
			b.WriteString("; ")
		}

		b.WriteString(err.Error())
	}

	return b.String()
}

func (e Errors) Check() error {
	if len(e) == 0 {
		return nil
	} else {
		return e
	}
}
