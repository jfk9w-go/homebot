package external

import (
	"context"
	"fmt"
	"homebot/common"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/httpf"
	"github.com/pkg/errors"
)

type Confirm func(ctx context.Context) (code string, err error)

type Client struct {
	httpClient *httpf.Client
	username   string
	sessionID  string
}

func NewClient(ctx context.Context, username string) (*Client, error) {
	httpClient := httpf.NewClient(nil).AcceptStatus(http.StatusOK)
	var r Response
	if err := httpClient.GET(SessionEndpoint).
		QueryParam("origin", origin).
		Context(ctx).
		Execute().
		DecodeBody(&r).
		Error; err != nil {
		return nil, errors.Wrap(err, "init session")
	}

	var sessionID string
	if err := r.Unmarshal("OK", &sessionID); err != nil {
		return nil, errors.Wrap(err, "init session")
	}

	return &Client{
		httpClient: httpClient,
		username:   username,
		sessionID:  sessionID,
	}, nil
}

func Authorize(ctx context.Context, username, password string, confirm Confirm) (*Client, error) {
	client, err := NewClient(ctx, username)
	if err != nil {
		return nil, errors.Wrap(err, "create client")
	}

	ticket, err := client.SignUp(ctx, password)
	if err != nil {
		return nil, errors.Wrap(err, "sign up")
	}

	code, err := confirm(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get answer")
	}

	if err := client.Confirm(ctx, ticket, "sign_up", strings.Trim(code, " \n\t")); err != nil {
		return nil, errors.Wrap(err, "confirm")
	}

	if err := client.LevelUp(ctx); err != nil {
		return nil, errors.Wrap(err, "level up")
	}

	return client, nil
}

func (c *Client) Username() string {
	return c.username
}

func (c *Client) SignUp(ctx context.Context, password string) (string, error) {
	var r Response
	if err := c.httpClient.POST(SignUpEndpoint).
		QueryParam("origin", origin).
		QueryParam("sessionid", c.sessionID).
		BodyEncoder(new(httpf.Form).
			Add("username", c.username).
			Add("password", password)).
		Context(ctx).
		Execute().
		DecodeBody(&r).
		Error; err != nil {
		return "", err
	}

	if err := r.Unmarshal("WAITING_CONFIRMATION", nil); err != nil {
		return "", err
	}

	return r.OperationTicket, nil
}

func (c *Client) Confirm(ctx context.Context, initialOperationTicket, initialOperation, code string) error {
	var r Response
	if err := c.httpClient.POST(ConfirmEndpoint).
		QueryParam("origin", origin).
		QueryParam("sessionid", c.sessionID).
		BodyEncoder(new(httpf.Form).
			Add("initialOperationTicket", initialOperationTicket).
			Add("initialOperation", initialOperation).
			Add("confirmationData", fmt.Sprintf(`{"SMSBYID":"%s"}`, code))).
		Context(ctx).
		Execute().
		DecodeBody(&r).
		Error; err != nil {
		return err
	}

	return r.Unmarshal("OK", nil)
}

func (c *Client) LevelUp(ctx context.Context) error {
	return c.httpClient.GET(LevelUpEndpoint).
		QueryParam("sessionid", c.sessionID).
		QueryParam("origin", origin).
		Context(ctx).
		Execute().
		Error
}

func (c *Client) Accounts(ctx context.Context) ([]Account, error) {
	var r Response
	if err := c.httpClient.POST(GroupedRequestsEndpount).
		QueryParam("_methods", "accounts_flat").
		QueryParam("sessionid", c.sessionID).
		BodyEncoder(new(httpf.Form).
			Add("requestsData", `[{"key":0,"operation":"accounts_flat"}]`)).
		Context(ctx).
		Execute().
		DecodeBody(&r).
		Error; err != nil {
		return nil, err
	}

	rs := make(map[string]Response)
	if err := r.Unmarshal("OK", &rs); err != nil {
		return nil, errors.Wrap(err, "decode responses")
	}

	accounts := make([]Account, 0)
	for _, r := range rs {
		as := make([]Account, 0)
		if err := r.Unmarshal("OK", &as); err != nil {
			return nil, errors.Wrap(err, "decode accounts")
		}

		accounts = append(accounts, as...)
	}

	return accounts, nil
}

func (c *Client) Operations(ctx context.Context, now time.Time, accountID string, since time.Time) ([]Operation, error) {
	formatTime := func(t time.Time) string { return strconv.FormatInt(t.UnixNano()/1e6, 10) }
	var r Response
	if err := c.httpClient.GET(OperationsEndpoint).
		QueryParam("sessionid", c.sessionID).
		QueryParam("account", accountID).
		QueryParam("start", formatTime(since)).
		QueryParam("end", formatTime(now)).
		Context(ctx).
		Execute().
		DecodeBody(&r).
		Error; err != nil {
		return nil, err
	}

	os := make([]Operation, 0)
	if err := r.Unmarshal("OK", &os); err != nil {
		return nil, err
	}

	sort.Sort(Operations(os))
	return os, nil
}

func (c *Client) ShoppingReceipt(ctx context.Context, operationID uint64) (*ShoppingReceipt, error) {
	var r Response
	if err := c.httpClient.GET(ShoppingReceiptEndpount).
		QueryParam("sessionid", c.sessionID).
		QueryParam("operationId", strconv.FormatUint(operationID, 10)).
		Context(ctx).
		Execute().
		DecodeBody(&r).
		Error; err != nil {
		return nil, err
	}

	receipt := new(ShoppingReceipt)
	if err := r.Unmarshal("OK", receipt); err != nil {
		if err, ok := err.(ResultCodeError); ok && err == "NO_DATA_FOUND" {
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
	return receipt, nil
}

func formatTime(t time.Time) string {
	if t.Before(TradingOperationsStart) {
		t = TradingOperationsStart
	}

	return t.Format("2006-01-02T15:04:05Z")
}

func (c *Client) TradingOperations(ctx context.Context, now time.Time, since time.Time) ([]TradingOperation, error) {
	var r Response
	if err := c.httpClient.POST(TradingOperationsEndpoint).
		QueryParam("sessionId", c.sessionID).
		BodyEncoder(flu.JSON(map[string]interface{}{
			"from":               formatTime(since),
			"to":                 formatTime(now),
			"overnightsDisabled": false,
		})).
		Context(ctx).
		Execute().
		DecodeBody(&r).
		Error; err != nil {
		return nil, err
	}

	var w struct {
		Items []TradingOperation `json:"items"`
	}

	if err := r.Unmarshal("OK", &w); err != nil {
		return nil, err
	}

	for i := range w.Items {
		w.Items[i].Username = c.username
	}

	return w.Items, nil
}

func (c *Client) PurchasedSecurities(ctx context.Context, now time.Time) ([]PurchasedSecurity, error) {
	var r Response
	if err := c.httpClient.POST(PurchasedSecuritiesEndpoint).
		QueryParam("sessionId", c.sessionID).
		BodyEncoder(flu.JSON(map[string]interface{}{
			"brokerAccountType": "Tinkoff",
			"currency":          "RUB",
		})).
		Context(ctx).
		Execute().
		DecodeBody(&r).
		Error; err != nil {
		return nil, err
	}

	var securities struct {
		Data []PurchasedSecurity `json:"data"`
	}

	if err := r.Unmarshal("OK", &securities); err != nil {
		return nil, err
	}

	for i := range securities.Data {
		securities.Data[i].Time = now
	}

	return securities.Data, nil
}

func (c *Client) Candles(ctx context.Context, ticker string, resolution interface{}, start, end time.Time) ([]Candle, error) {
	start = common.TrimDate(start.Add(-24 * time.Hour))
	end = common.TrimDate(end.Add(24 * time.Hour))

	var r Response
	if err := c.httpClient.POST(PurchasedSecuritiesEndpoint).
		QueryParam("sessionId", c.sessionID).
		BodyEncoder(flu.JSON(map[string]interface{}{
			"from":       formatTime(start),
			"end":        formatTime(end),
			"ticker":     ticker,
			"resolution": resolution,
		})).
		Context(ctx).
		Execute().
		DecodeBody(&r).
		Error; err != nil {
		return nil, err
	}

	var candles struct {
		Candles []Candle `json:"candles"`
	}

	if err := r.Unmarshal("Ok", &candles); err != nil {
		return nil, err
	}

	for i := range candles.Candles {
		(&candles.Candles[i]).Ticker = ticker
	}

	return candles.Candles, nil
}
