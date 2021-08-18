package tinkoff

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jfk9w-go/flu"
	fluhttp "github.com/jfk9w-go/flu/http"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Client struct {
	Auth
	Username   string
	HttpClient *fluhttp.Client
	wg         *flu.WaitGroup
	cancel     func()
}

func (c *Client) PingInBackground(ctx context.Context, each time.Duration) {
	log := logrus.WithField("username", c.Username)
	if c.wg != nil {
		log.Warnf("background ping already running")
		return
	}

	c.wg = new(flu.WaitGroup)
	c.cancel = c.wg.Go(ctx, func(ctx context.Context) {
		ticker := time.NewTicker(each)
		defer func() {
			ticker.Stop()
			if err := ctx.Err(); err != nil {
				log.Warnf("ping context canceled: %s", err)
			}
		}()

		for {
			if err := c.Ping(ctx); err != nil {
				if errors.Is(err, ErrInvalidAccessLevel) {
					log.Fatalf("session ID expired: %s", err)
				} else if ctx.Err() != nil {
					return
				} else {
					log.Warnf("ping: %s", err)
				}
			} else {
				log.Debugf("ping ok")
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})

	log.Infof("started background ping")
}

func (c *Client) Close() error {
	if c.wg != nil {
		c.cancel()
		c.wg.Wait()
	}

	return nil
}

var ErrInvalidAccessLevel = errors.New("invalid access level")

func (c *Client) Ping(ctx context.Context) error {
	sessionID, err := c.SessionID()
	if err != nil {
		return errors.Wrap(err, "auth")
	}

	var r response
	if err := c.HttpClient.GET(PingEndpoint).
		QueryParam("sessionid", sessionID).
		Context(ctx).
		Execute().
		CheckStatus(http.StatusOK).
		DecodeBody(flu.JSON{Value: &r}).
		Error; err != nil {
		return err
	}

	var data struct {
		AccessLevel string `json:"accessLevel"`
	}

	if err := r.decode(&data); err != nil {
		return err
	}

	if data.AccessLevel != "CLIENT" {
		return errors.Wrap(err, data.AccessLevel)
	}

	return nil
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

func (c *Client) Operations(ctx context.Context, now time.Time, accountID string, since time.Time) ([]Operation, error) {
	sessionID, err := c.SessionID()
	if err != nil {
		return nil, errors.Wrap(err, "auth")
	}

	formatTime := func(t time.Time) string { return strconv.FormatInt(t.UnixNano()/1e6, 10) }
	var r response
	if err := c.HttpClient.GET(OperationsEndpoint).
		QueryParam("sessionid", sessionID).
		QueryParam("account", accountID).
		QueryParam("start", formatTime(since)).
		QueryParam("end", formatTime(now)).
		Context(ctx).
		Execute().
		CheckStatus(http.StatusOK).
		DecodeBody(flu.JSON{Value: &r}).
		Error; err != nil {
		return nil, err
	}

	operations := make([]Operation, 0)
	if err := r.decode(&operations); err != nil {
		return nil, err
	}

	sort.Sort(operationSort(operations))
	return operations, nil
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

func (c *Client) TradingOperations(ctx context.Context, now time.Time, since time.Time) ([]TradingOperation, error) {
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
			"from":               formatTime(since),
			"to":                 formatTime(now),
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

	if err := r.decode(&w); err != nil {
		return nil, err
	}

	for i := range w.Items {
		w.Items[i].Username = c.Username
	}

	return w.Items, nil
}

func (c *Client) PurchasedSecurities(ctx context.Context, now time.Time) ([]PurchasedSecurity, error) {
	sessionID, err := c.SessionID()
	if err != nil {
		return nil, errors.Wrap(err, "auth")
	}

	var r response
	if err := c.HttpClient.POST(PurchasedSecuritiesEndpoint).
		QueryParam("sessionId", sessionID).
		BodyEncoder(flu.JSON{Value: map[string]interface{}{
			"brokerAccountType": "Tinkoff",
			"currency":          "RUB",
		}}).
		Context(ctx).
		Execute().
		CheckStatus(http.StatusOK).
		DecodeBody(flu.JSON{Value: &r}).
		Error; err != nil {
		return nil, err
	}

	var securities struct {
		Data []PurchasedSecurity `json:"data"`
	}

	if err := r.decode(&securities); err != nil {
		return nil, err
	}

	for i := range securities.Data {
		securities.Data[i].Time = now
	}

	return securities.Data, nil
}
