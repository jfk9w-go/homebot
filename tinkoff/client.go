package tinkoff

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/jfk9w-go/flu"
	fluhttp "github.com/jfk9w-go/flu/http"
	"github.com/pkg/errors"
)

type Client struct {
	Auth
	HttpClient *fluhttp.Client
}

func (c *Client) Accounts(ctx context.Context) ([]Account, error) {
	sessionID, err := c.SessionID()
	if err != nil {
		return nil, errors.Wrap(err, "auth")
	}

	var r response
	if err := c.HttpClient.POST(GroupedRequestsEndpount).
		QueryParam("_methods", "accounts_flat").
		QueryParam("sessionid", sessionID).
		BodyEncoder(new(fluhttp.Form).
			Add("requestsData", `[{"key":0,"operation":"accounts_flat"}]`)).
		Context(ctx).
		Execute().
		CheckStatus(http.StatusOK).
		DecodeBody(flu.JSON{Value: &r}).
		Error; err != nil {
		return nil, err
	}

	rs := make(map[string]response)
	if err := r.decode(&rs); err != nil {
		return nil, errors.Wrap(err, "decode responses")
	}

	accounts := make([]Account, 0)
	for _, r := range rs {
		as := make([]Account, 0)
		if err := r.decode(&as); err != nil {
			return nil, errors.Wrap(err, "decode accounts")
		}

		accounts = append(accounts, as...)
	}

	return accounts, nil
}

func (c *Client) Operations(ctx context.Context, accountID string, from, to time.Time) ([]Operation, error) {
	sessionID, err := c.SessionID()
	if err != nil {
		return nil, errors.Wrap(err, "auth")
	}

	formatTime := func(t time.Time) string { return strconv.FormatInt(t.UnixNano()/1e6, 10) }
	var r response
	if err := c.HttpClient.GET(OperationsEndpoint).
		QueryParam("sessionid", sessionID).
		QueryParam("account", accountID).
		QueryParam("start", formatTime(from)).
		QueryParam("end", formatTime(to)).
		Context(ctx).
		Execute().
		CheckStatus(http.StatusOK).
		DecodeBody(flu.JSON{Value: &r}).
		Error; err != nil {
		return nil, err
	}

	items := make([]Operation, 0)
	return items, r.decode(&items)
}

func (c *Client) ShoppingReceipt(ctx context.Context, operationID uint64) (*ShoppingReceipt, error) {
	sessionID, err := c.SessionID()
	if err != nil {
		return nil, errors.Wrap(err, "auth")
	}

	var r response
	if err := c.HttpClient.GET(ShoppingReceiptEndpount).
		QueryParam("sessionid", sessionID).
		QueryParam("operationId", strconv.FormatUint(operationID, 10)).
		Context(ctx).
		Execute().
		CheckStatus(http.StatusOK).
		DecodeBody(flu.JSON{Value: &r}).
		Error; err != nil {
		return nil, err
	}

	receipt := new(ShoppingReceipt)
	if err := r.decode(receipt); err != nil {
		if err, ok := err.(responseError); ok && err == "NO_DATA_FOUND" {
			return nil, nil
		}

		return nil, err
	}

	type itemKey struct {
		Name  string
		Price float64
	}

	itemsByPrimaryKey := make(map[itemKey]ShoppingReceiptItem)
	for _, item := range receipt.Receipt.Items {
		key := itemKey{item.Name, item.Price}
		if previous, ok := itemsByPrimaryKey[key]; ok {
			item.Quantity += previous.Quantity
			item.Sum += previous.Sum
		}

		itemsByPrimaryKey[key] = item
	}

	items := make([]ShoppingReceiptItem, len(itemsByPrimaryKey))
	i := 0
	for _, item := range itemsByPrimaryKey {
		items[i] = item
		i++
	}

	receipt.Receipt.Items = items
	return receipt, err
}

func (c *Client) TradingOperations(ctx context.Context, from, to time.Time) ([]TradingOperation, error) {
	sessionID, err := c.SessionID()
	if err != nil {
		return nil, errors.Wrap(err, "auth")
	}

	formatTime := func(t time.Time) string {
		if t.Before(tradingOperationsStart) {
			t = tradingOperationsStart
		}

		return t.Format("2006-01-02T15:04:05Z")
	}

	var r response
	if err := c.HttpClient.POST(TradingOperationsEndpoint).
		QueryParam("sessionId", sessionID).
		BodyEncoder(flu.JSON{Value: map[string]interface{}{
			"from":               formatTime(from),
			"to":                 formatTime(to),
			"overnightsDisabled": false,
		}}).
		Context(ctx).
		Execute().
		CheckStatus(http.StatusOK).
		DecodeBody(flu.JSON{Value: &r}).
		Error; err != nil {
		return nil, err
	}

	var w struct {
		Items []TradingOperation `json:"items"`
	}

	return w.Items, r.decode(&w)
}
