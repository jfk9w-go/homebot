package tinkoff

import (
	"encoding/json"
	"strings"
	"time"
)

var (
	Host                        = "https://www.tinkoff.ru"
	LoginPage                   = Host + "/login/"
	PingEndpoint                = Host + "/api/common/v1/ping"
	GroupedRequestsEndpount     = Host + "/api/common/v1/grouped_requests"
	OperationsEndpoint          = Host + "/api/common/v1/operations"
	ShoppingReceiptEndpount     = Host + "/api/common/v1/shopping_receipt"
	TradingOperationsEndpoint   = Host + "/api/trading/user/operations"
	PurchasedSecuritiesEndpoint = Host + "/api/trading/portfolio/purchased_securities"

	tradingOperationsStart = time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)
)

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

type operationSort []Operation

func (os operationSort) Len() int {
	return len(os)
}

func (os operationSort) Less(i, j int) bool {
	return os[i].ID < os[j].ID
}

func (os operationSort) Swap(i, j int) {
	os[i], os[j] = os[j], os[i]
}
