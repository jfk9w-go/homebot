package external

import (
	"context"
	"fmt"
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

type ClientConfig struct {
	Username string
	Phone    string
	Password string
}

type Client struct {
	httpf.Client
	username string
}

func NewClient(ctx context.Context, username string) (*Client, error) {
	var r Response
	if err := httpf.GET(SessionEndpoint).
		Query("origin", origin).
		Exchange(ctx, nil).
		DecodeBody(&r).
		Error(); err != nil {
		return nil, errors.Wrap(err, "init session")
	}

	var sessionID string
	if err := r.Unmarshal("OK", &sessionID); err != nil {
		return nil, errors.Wrap(err, "init session")
	}

	return &Client{
		Client: &http.Client{
			Transport: withQueryParams(httpf.NewDefaultTransport(), sessionID),
		},
		username: username,
	}, nil
}

func Authorize(ctx context.Context, cred Credential, confirm Confirm) (*Client, error) {
	client, err := NewClient(ctx, cred.Username)
	if err != nil {
		return nil, errors.Wrap(err, "create client")
	}

	ticket, err := client.SignUp(ctx, "WAITING_CONFIRMATION", new(httpf.Form).Set("phone", cred.Phone))
	if err != nil {
		return nil, errors.Wrap(err, "sign in with phone")
	}

	code, err := confirm(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get answer")
	}

	if err := client.Confirm(ctx, ticket, "sign_up", strings.Trim(code, " \n\t")); err != nil {
		return nil, errors.Wrap(err, "confirm")
	}

	if _, err := client.SignUp(ctx, "OK", new(httpf.Form).Set("password", cred.Password)); err != nil {
		return nil, errors.Wrap(err, "sign in with password")
	}

	if err := client.LevelUp(ctx); err != nil {
		return nil, errors.Wrap(err, "level up")
	}

	return client, nil
}

func (c *Client) Username() string {
	return c.username
}

func (c *Client) SignUp(ctx context.Context, expectedStatusCode string, body flu.EncoderTo) (string, error) {
	var r Response
	if err := httpf.POST(SignUpEndpoint, body).
		Exchange(ctx, c).
		DecodeBody(&r).
		Error(); err != nil {
		return "", err
	}

	if err := r.Unmarshal(expectedStatusCode, nil); err != nil {
		return "", err
	}

	return r.OperationTicket, nil
}

func (c *Client) Confirm(ctx context.Context, initialOperationTicket, initialOperation, code string) error {
	var r Response
	if err := httpf.POST(ConfirmEndpoint,
		new(httpf.Form).
			Set("initialOperationTicket", initialOperationTicket).
			Set("initialOperation", initialOperation).
			Set("confirmationData", fmt.Sprintf(`{"SMSBYID":"%s"}`, code))).
		Exchange(ctx, c).
		DecodeBody(&r).
		Error(); err != nil {
		return err
	}

	return r.Unmarshal("OK", nil)
}

func (c *Client) LevelUp(ctx context.Context) error {
	return httpf.GET(LevelUpEndpoint).
		Exchange(ctx, c).
		Error()
}

func (c *Client) Accounts(ctx context.Context) ([]Account, error) {
	var r Response
	if err := httpf.POST(GroupedRequestsEndpount,
		new(httpf.Form).
			Set("requestsData", `[{"key":0,"operation":"accounts_flat"}]`)).
		Query("_methods", "accounts_flat").
		Exchange(ctx, c).
		DecodeBody(&r).
		Error(); err != nil {
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
	if err := httpf.GET(OperationsEndpoint).
		Query("account", accountID).
		Query("start", formatTime(since)).
		Query("end", formatTime(now)).
		Exchange(ctx, c).
		DecodeBody(&r).
		Error(); err != nil {
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
	if err := httpf.GET(ShoppingReceiptEndpount).
		Query("operationId", strconv.FormatUint(operationID, 10)).
		Exchange(ctx, c).
		DecodeBody(&r).
		Error(); err != nil {
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
	if err := httpf.POST(TradingOperationsEndpoint,
		flu.JSON(map[string]interface{}{
			"from":               formatTime(since),
			"to":                 formatTime(now),
			"overnightsDisabled": false,
		})).
		Exchange(ctx, c).
		DecodeBody(&r).
		Error(); err != nil {
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
	if err := httpf.POST(PurchasedSecuritiesEndpoint,
		flu.JSON(map[string]interface{}{
			"brokerAccountType": "Tinkoff",
			"currency":          "RUB",
		})).
		Exchange(ctx, c).
		DecodeBody(&r).
		Error(); err != nil {
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

const MaxCandlePeriod = 12 * 30 * 24 * time.Hour // 1 year

func (c *Client) Candles(ctx context.Context, ticker string, resolution interface{}, start, end time.Time) ([]Candle, error) {
	candles := make([]Candle, 0)
	for {
		var (
			localEnd = end
			r        Response
		)

		if end.Sub(start) > MaxCandlePeriod {
			localEnd = start.Add(MaxCandlePeriod)
		}

		if err := httpf.POST(CandlesEndpoint,
			flu.JSON(map[string]interface{}{
				"from":       formatTime(start),
				"to":         formatTime(localEnd),
				"ticker":     ticker,
				"resolution": resolution,
			})).
			Exchange(ctx, c).
			DecodeBody(&r).
			Error(); err != nil {
			return nil, err
		}

		var resp struct {
			Candles []Candle `json:"candles"`
		}

		if err := r.Unmarshal("Ok", &resp); err != nil {
			return nil, err
		}

		for i := range resp.Candles {
			(&resp.Candles[i]).Ticker = ticker
		}

		candles = append(candles, resp.Candles...)
		if localEnd == end {
			break
		}

		start = localEnd
	}

	return candles, nil
}

func withQueryParams(rt http.RoundTripper, sessionID string) httpf.RoundTripperFunc {
	return func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		query.Set("sessionId", sessionID)
		query.Set("sessionid", sessionID)
		query.Set("origin", origin)
		req.URL.RawQuery = query.Encode()
		return rt.RoundTrip(req)
	}
}
