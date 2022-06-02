package tinkoff

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/jfk9w-go/flu/colf"
	"github.com/jfk9w-go/flu/logf"
	"github.com/pkg/errors"
)

type InvalidResultCode struct {
	Expected, Actual string
	Message          string
}

func (e InvalidResultCode) Error() string {
	return fmt.Sprintf("expected result [%s], got [%s] with message [%s]", e.Expected, e.Actual, e.Message)
}

type commonResponse[P any] struct {
	ResultCode      string `json:"resultCode,omitempty"`
	ErrorMessage    string `json:"errorMessage,omitempty"`
	PlainMessage    string `json:"plainMessage,omitempty"`
	Payload         P      `json:"payload,omitempty"`
	OperationTicket string `json:"operationTicket,omitempty"`
}

func (r commonResponse[P]) validate(expectedResultCode string) error {
	switch r.ResultCode {
	case "REQUEST_RATE_LIMIT_EXCEEDED":
		return ErrRequestRateLimitExceeded
	case "INSUFFICIENT_PRIVILEGES":
		return ErrInsufficientPrivileges
	case "NO_DATA_FOUND":
		return ErrNoDataFound
	case expectedResultCode:
		return nil
	default:
		return InvalidResultCode{
			Expected: expectedResultCode,
			Actual:   r.ResultCode,
			Message:  r.ErrorMessage,
		}
	}
}

type tradingError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e tradingError) Error() string {
	return fmt.Sprintf("%s [%s]", e.Code, e.Message)
}

type tradingResponse[P any] struct {
	Status  string          `json:"status"`
	Payload json.RawMessage `json:"payload"`
}

func (r tradingResponse[P]) validate(expectedStatus string) error {
	if r.Status == expectedStatus {
		return nil
	}

	var tradingErr tradingError
	if r.Status == "Error" {
		if err := json.Unmarshal(r.Payload, &tradingErr); err != nil {
			logf.Warnf(nil, "failed to unmarshal trading response [%s] into error: %v", string(r.Payload), err)
		} else {
			switch tradingErr.Code {
			case "InsufficientPrivileges":
				return ErrInsufficientPrivileges
			default:
				return tradingErr
			}
		}
	}

	return InvalidResultCode{
		Expected: expectedStatus,
		Actual:   r.Status,
	}
}

type Unit = any

type exchange[R any] interface {
	response() R
}

type commonExchange[R any] interface {
	exchange[R]
	operation() string
	resultCode() string
}

type groupedRequests[R any] colf.Slice[commonExchange[R]]

func (groupedRequests[R]) operation() string                 { return "grouped_requests" }
func (groupedRequests[R]) resultCode() string                { return "OK" }
func (groupedRequests[R]) response() (r []commonResponse[R]) { return }

type groupedRequestData struct {
	Key       int    `json:"key"`
	Operation string `json:"operation"`
	Params    any    `json:"params,omitempty"`
}

func (gr groupedRequests[R]) EncodeValues(key string, v *url.Values) error {
	requests := make([]groupedRequestData, len(gr))
	for i, exchange := range gr {
		requests[i] = groupedRequestData{
			Key:       i,
			Operation: exchange.operation(),
			Params:    exchange,
		}
	}

	requestsData, err := json.Marshal(requests)
	if err != nil {
		return err
	}

	v.Set("requestsData", string(requestsData))
	return nil
}

type session struct{}

func (session) operation() string    { return "session" }
func (session) resultCode() string   { return "OK" }
func (session) response() (_ string) { return }

type ping struct{}

type pong struct {
	AccessLevel string `json:"accessLevel"`
}

func (ping) operation() string  { return "ping" }
func (ping) resultCode() string { return "OK" }
func (ping) response() (_ pong) { return }

type signUp struct{}

func (signUp) operation() string  { return "sign_up" }
func (signUp) response() (_ Unit) { return }

type phoneSignUp struct {
	signUp
	Phone string `url:"phone"`
}

func (phoneSignUp) resultCode() string { return "WAITING_CONFIRMATION" }

type passwordSignUp struct {
	signUp
	Password string `url:"password"`
}

func (passwordSignUp) resultCode() string { return "OK" }

type confirmationData struct {
	SMSBYID string `json:"SMSBYID"`
}

type confirm struct {
	initialOperation       string
	initialOperationTicket string
	confirmationData       confirmationData
}

func (confirm) operation() string  { return "confirm" }
func (confirm) resultCode() string { return "OK" }
func (confirm) response() (_ Unit) { return }

func (e confirm) EncodeValues(key string, v *url.Values) error {
	confirmationData, err := json.Marshal(e.confirmationData)
	if err != nil {
		return err
	}

	v.Set("initialOperation", e.initialOperation)
	v.Set("initialOperationTicket", e.initialOperationTicket)
	v.Set("confirmationData", string(confirmationData))

	return nil
}

type levelUp struct{}

func (levelUp) operation() string  { return "level_up" }
func (levelUp) resultCode() string { return "OK" }
func (levelUp) response() (_ Unit) { return }

type Accounts struct{}

func (Accounts) operation() string       { return "accounts_flat" }
func (Accounts) resultCode() string      { return "OK" }
func (Accounts) response() (_ []Account) { return }

type Operations struct {
	AccountID  string
	Start, End time.Time
}

func (Operations) operation() string         { return "operations" }
func (Operations) resultCode() string        { return "OK" }
func (Operations) response() (_ []Operation) { return }

func (o Operations) EncodeValues(key string, v *url.Values) error {
	if o.AccountID == "" {
		return errors.New("account is required")
	}

	v.Set("start", fmt.Sprint(o.Start.UnixMilli()))
	if !o.End.IsZero() {
		v.Set("end", fmt.Sprint(o.End.UnixMilli()))
	}

	v.Set("account", o.AccountID)
	return nil
}

type OperationReceipt struct {
	OperationID uint64 `url:"operationId"`
}

func (OperationReceipt) operation() string             { return "shopping_receipt" }
func (OperationReceipt) resultCode() string            { return "OK" }
func (OperationReceipt) response() (_ ShoppingReceipt) { return }

type tradingExchange[R any] interface {
	exchange[R]
	path() string
}

type TradingOperations struct {
	From, To           time.Time
	OvernightsDisabled bool
}

var tradingTimeStart = time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)

type TradingOperationsResponse struct {
	Items []TradingOperation `json:"items"`
}

func (TradingOperations) path() string                            { return "/user/operations" }
func (TradingOperations) response() (_ TradingOperationsResponse) { return }

func formatTradingTime(value time.Time) string {
	return value.Format("2006-01-02T15:04:05Z")
}

func (tos TradingOperations) MarshalJSON() ([]byte, error) {
	from := tos.From
	if from.IsZero() {
		from = tradingTimeStart
	}

	to := tos.To
	if to.IsZero() {
		to = time.Now()
	}

	return json.Marshal(map[string]any{
		"from":               formatTradingTime(from),
		"to":                 formatTradingTime(to),
		"overnightsDisabled": tos.OvernightsDisabled,
	})
}

var TinkoffRUB = PurchasedSecurities{
	BrokerAccountType: "Tinkoff",
	Currency:          "RUB",
}

type PurchasedSecurities struct {
	BrokerAccountType string `json:"brokerAccountType"`
	Currency          string `json:"currency"`
}

type purchasedSecuritiesResponse struct {
	Data []PurchasedSecurity `json:"data"`
}

func (PurchasedSecurities) path() string                              { return "/portfolio/purchased_securities" }
func (PurchasedSecurities) response() (_ purchasedSecuritiesResponse) { return }

type Candles struct {
	Ticker     string
	Resolution any
	From, To   time.Time
}

type candlesResponse struct {
	Candles []Candle `json:"candles"`
}

func (Candles) path() string                  { return "/symbols/candles" }
func (Candles) response() (_ candlesResponse) { return }

func (cs Candles) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"from":       formatTradingTime(cs.From),
		"to":         formatTradingTime(cs.To),
		"ticker":     cs.Ticker,
		"resolution": cs.Resolution,
	})
}
