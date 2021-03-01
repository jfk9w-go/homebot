package tinkoff

import (
	"encoding/json"
	"strings"
	"time"
)

var (
	Host                      = "https://www.tinkoff.ru"
	LoginPage                 = Host + "/login/"
	GroupedRequestsEndpount   = Host + "/api/common/v1/grouped_requests"
	OperationsEndpoint        = Host + "/api/common/v1/operations"
	ShoppingReceiptEndpount   = Host + "/api/common/v1/shopping_receipt"
	TradingOperationsEndpoint = Host + "/api/trading/user/operations"

	tradingOperationsStart = time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)
	moscowLocation         *time.Location
)

func init() {
	var err error
	moscowLocation, err = time.LoadLocation("Europe/Moscow")
	if err != nil {
		panic(err)
	}
}

type response struct {
	ResultCode string          `json:"resultCode"`
	Status     string          `json:"status"`
	Payload    json.RawMessage `json:"payload"`
}

func (r response) decode(value interface{}) error {
	code := r.ResultCode
	if code == "" {
		code = r.Status
	}

	if strings.ToUpper(code) != "OK" {
		return responseError(code)
	}

	return json.Unmarshal(r.Payload, value)
}

type responseError string

func (e responseError) Error() string {
	return string(e)
}
