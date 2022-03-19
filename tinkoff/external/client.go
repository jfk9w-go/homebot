package external

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

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
	limiter  flu.RateLimiter
}

func NewClient(ctx context.Context, username string) (*Client, error) {
	httpClient := &http.Client{
		Transport: httpf.NewDefaultTransport(),
	}

	c := &Client{
		Client:   httpClient,
		username: username,
		limiter:  flu.IntervalRateLimiter(time.Second),
	}

	var (
		req = httpf.GET(SessionEndpoint).
			Query("origin", origin)

		sessionID string
	)

	if _, err := c.exchange(ctx, req, "OK", &sessionID); err != nil {
		return nil, errors.Wrap(err, "init session")
	}

	httpClient.Transport = withQueryParams(httpClient.Transport, sessionID)
	return c, nil
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
	req := httpf.POST(SignUpEndpoint, body)
	operationTicket, err := c.exchange(ctx, req, expectedStatusCode, nil)
	if err != nil {
		return "", errors.Wrap(err, "exchange")
	}

	return operationTicket, nil
}

func (c *Client) Confirm(ctx context.Context, initialOperationTicket, initialOperation, code string) error {
	req := httpf.POST(ConfirmEndpoint,
		new(httpf.Form).
			Set("initialOperationTicket", initialOperationTicket).
			Set("initialOperation", initialOperation).
			Set("confirmationData", fmt.Sprintf(`{"SMSBYID":"%s"}`, code)))

	if _, err := c.exchange(ctx, req, "OK", nil); err != nil {
		return errors.Wrap(err, "exchange")
	}

	return nil
}

func (c *Client) LevelUp(ctx context.Context) error {
	return httpf.GET(LevelUpEndpoint).
		Exchange(ctx, c).
		Error()
}

func (c *Client) Accounts(ctx context.Context) ([]Account, error) {
	var (
		req = httpf.POST(GroupedRequestsEndpount,
			new(httpf.Form).
				Set("requestsData", `[{"key":0,"operation":"accounts_flat"}]`)).
			Query("_methods", "accounts_flat")

		resp map[string]response
	)

	if _, err := c.exchange(ctx, req, "OK", &resp); err != nil {
		return nil, errors.Wrap(err, "exchange")
	}

	accounts := make([]Account, 0, len(resp))
	for _, r := range resp {
		as := make([]Account, 0)
		if err := json.Unmarshal(r.Payload, &as); err != nil {
			return nil, errors.Wrap(err, "decode accounts")
		}

		accounts = append(accounts, as...)
	}

	return accounts, nil
}

func (c *Client) Operations(ctx context.Context, now time.Time, accountID string, since time.Time) ([]Operation, error) {
	var (
		formatTime = func(t time.Time) string {
			return strconv.FormatInt(t.UnixNano()/1e6, 10)
		}

		req = httpf.GET(OperationsEndpoint).
			Query("account", accountID).
			Query("start", formatTime(since)).
			Query("end", formatTime(now))

		resp = make([]Operation, 0)
	)

	if _, err := c.exchange(ctx, req, "OK", &resp); err != nil {
		return nil, errors.Wrap(err, "exchange")
	}

	sort.Sort(Operations(resp))
	return resp, nil
}

func (c *Client) ShoppingReceipt(ctx context.Context, operationID uint64) (*ShoppingReceipt, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var (
		req = httpf.GET(ShoppingReceiptEndpount).
			Query("operationId", strconv.FormatUint(operationID, 10))

		resp ShoppingReceipt
	)

	if _, err := c.exchange(ctx, req, "OK", &resp); err != nil {
		if err, ok := err.(ResultCodeError); ok && err == "NO_DATA_FOUND" {
			return nil, nil
		}

		return nil, errors.Wrap(err, "exchange")
	}

	type itemKey struct {
		Name  string
		Price float64
	}

	itemsByPrimaryKey := make(map[itemKey]ShoppingReceiptItem)
	for _, item := range resp.Receipt.Items {
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

	resp.Receipt.Items = items
	return &resp, nil
}

func formatTime(t time.Time) string {
	if t.Before(TradingOperationsStart) {
		t = TradingOperationsStart
	}

	return t.Format("2006-01-02T15:04:05Z")
}

func (c *Client) TradingOperations(ctx context.Context, now time.Time, since time.Time) ([]TradingOperation, error) {
	var (
		req = httpf.POST(TradingOperationsEndpoint, flu.JSON(object{
			"from":               formatTime(since),
			"to":                 formatTime(now),
			"overnightsDisabled": false,
		}))

		resp struct {
			Items []TradingOperation `json:"items"`
		}
	)

	if _, err := c.exchange(ctx, req, "OK", &resp); err != nil {
		return nil, errors.Wrap(err, "exchange")
	}

	for i := range resp.Items {
		resp.Items[i].Username = c.username
	}

	return resp.Items, nil
}

func (c *Client) PurchasedSecurities(ctx context.Context, now time.Time) ([]PurchasedSecurity, error) {
	var (
		req = httpf.POST(PurchasedSecuritiesEndpoint, flu.JSON(object{
			"brokerAccountType": "Tinkoff",
			"currency":          "RUB",
		}))

		resp struct {
			Data []PurchasedSecurity `json:"data"`
		}
	)

	if _, err := c.exchange(ctx, req, "OK", &resp); err != nil {
		return nil, errors.Wrap(err, "exchange")
	}

	for i := range resp.Data {
		resp.Data[i].Time = now
	}

	return resp.Data, nil
}

const MaxCandlePeriod = 12 * 30 * 24 * time.Hour // 1 year

func (c *Client) Candles(ctx context.Context, ticker string, resolution interface{}, start, end time.Time) ([]Candle, error) {
	candles := make([]Candle, 0)
	for {
		var localEnd = end
		if end.Sub(start) > MaxCandlePeriod {
			localEnd = start.Add(MaxCandlePeriod)
		}

		var (
			req = httpf.POST(CandlesEndpoint, flu.JSON(object{
				"from":       formatTime(start),
				"to":         formatTime(localEnd),
				"ticker":     ticker,
				"resolution": resolution,
			}))

			resp struct {
				Candles []Candle `json:"candles"`
			}
		)

		if _, err := c.exchange(ctx, req, "OK", &resp); err != nil {
			return nil, errors.Wrap(err, "exchange")
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

func (c *Client) exchange(ctx context.Context, req *httpf.RequestBuilder, expectedStatus string, result interface{}) (string, error) {
	if err := c.limiter.Start(ctx); err != nil {
		return "", err
	}

	defer c.limiter.Complete()
	return c.exchange0(ctx, req, expectedStatus, result)
}

func (c *Client) exchange0(ctx context.Context, req *httpf.RequestBuilder, expectedStatus string, result interface{}) (string, error) {
	var (
		resp response
		log  = c.log().WithFields(logrus.Fields{
			"method": req.Request.Method,
			"url":    req.URL.String(),
		})
	)

	if err := req.Exchange(ctx, c).DecodeBody(flu.JSON(&resp)).Error(); err != nil {
		return "", err
	}

	status := resp.ResultCode
	if status == "" {
		status = resp.Status
	}

	switch strings.ToUpper(status) {
	case expectedStatus:
		if result != nil {
			if err := json.Unmarshal(resp.Payload, result); err != nil {
				return "", errors.Wrap(err, "unmarshal payload")
			}
		}

		log.Debugf("exchange ok")
		return resp.OperationTicket, nil
	case "REQUEST_RATE_LIMIT_EXCEEDED":
		log.Warnf("rate limit exceeded, retrying exchange")
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Second):
			return c.exchange0(ctx, req, expectedStatus, result)
		}
	default:
		return "", ResultCodeError(status)
	}
}

func (c *Client) log() *logrus.Entry {
	return logrus.WithField("client", "tinkoff")
}

type object map[string]interface{}

type response struct {
	ResultCode      string          `json:"resultCode"`
	Status          string          `json:"status"`
	Payload         json.RawMessage `json:"payload"`
	OperationTicket string          `json:"operationTicket"`
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
