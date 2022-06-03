package tinkoff

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/apfel"
	"github.com/jfk9w-go/flu/colf"
	"github.com/jfk9w-go/flu/httpf"
	"github.com/jfk9w-go/flu/logf"
	"github.com/jfk9w-go/flu/syncf"
	"github.com/pkg/errors"
)

var (
	ErrRequestRateLimitExceeded = errors.New("request rate limit exceeded, please try again later")
	ErrInsufficientPrivileges   = errors.New("insufficient privileges")
	ErrNoDataFound              = errors.New("no data found")
)

type Credential struct {
	Username  string `yaml:"username" doc:"Username is used for operation distinction."`
	Phone     string `yaml:"phone" doc:"Phone is required to login."`
	Password  string `yaml:"password" doc:"Password is required to login."`
	SessionID string `yaml:"sessionId,omitempty" doc:"If set, no user authorization will be required while provided session ID is valid."`
}

func (cr Credential) String() string {
	return "tinkoff.client." + cr.Username
}

type ConfirmFunc func(ctx context.Context, username string) (code string, err error)

type Client[C any] struct {
	Credential
	client *client
	clock  syncf.Clock
}

func (c Client[C]) String() string {
	return c.Credential.String()
}

func (c *Client[C]) Include(ctx context.Context, app apfel.MixinApp[C]) error {
	var confirmFunc apfel.MixinAny[C, ConfirmFunc]
	if err := app.Use(ctx, &confirmFunc, true); err != nil {
		return err
	}

	c.clock = app
	c.client = &client{
		Credential: c.Credential,
		confirm:    confirmFunc.Value,
		client: &http.Client{
			Transport: httpf.NewDefaultTransport(),
		},
		commonsMu: map[string]syncf.Locker{
			"shopping_receipt": syncf.Lockers{
				syncf.Semaphore(app, 25, 75*time.Second),
				syncf.Semaphore(app, 75, 11*time.Minute),
			},
		},
	}

	if sessionID := c.Credential.SessionID; sessionID != "" {
		c.client.sessionID = sessionID
		c.client.ping()
		if err := app.Manage(ctx, sessionLogger(func() {
			ctx := context.Background()
			sessionID, _ := c.client.getSessionID(ctx)
			logf.Get(c).Infof(ctx, "current session ID: [%s]", sessionID)
		})); err != nil {
			return err
		}
	}

	if err := app.Manage(ctx, c); err != nil {
		return err
	}

	return nil
}

type sessionLogger func()

func (l sessionLogger) Close() error {
	l()
	return nil
}

func (c *Client[C]) GetAccounts(ctx context.Context, req Accounts) ([]Account, error) {
	all, err := executeAuthorizedExchange[[]Account](ctx, c.client, req)
	if err != nil {
		return nil, err
	}

	var accounts []Account
	for _, account := range all {
		if account.Type == "SharedCredit" ||
			account.Type == "SharedCurrent" ||
			account.Type == "ExternalAccount" {
			continue
		}

		account.Username = c.Username
		accounts = append(accounts, account)
	}

	return accounts, nil
}

func (c *Client[C]) GetOperations(ctx context.Context, req Operations) ([]Operation, error) {
	operations, err := executeAuthorizedExchange[[]Operation](ctx, c.client, req)
	if err != nil {
		return nil, err
	}

	sort.Sort(operationSort(operations))

	for i, operation := range operations {
		locations := make(colf.Set[OperationLocation], len(operation.Locations))
		for _, location := range operation.Locations {
			locations.Add(location)
		}

		operations[i].Locations = colf.ToSlice[OperationLocation](locations)
	}

	return operations, nil
}

type shoppingReceiptItemKey struct {
	name  string
	price float64
}

func (c *Client[C]) GetShoppingReceipt(ctx context.Context, req OperationReceipt) (*ShoppingReceipt, error) {
	receipt, err := executeAuthorizedExchange[ShoppingReceipt](ctx, c.client, req)
	if err != nil {
		return nil, err
	}

	itemsByPrimaryKey := make(map[shoppingReceiptItemKey]ShoppingReceiptItem)
	for _, item := range receipt.Receipt.Items {
		key := shoppingReceiptItemKey{item.Name, item.Price}
		if previous, ok := itemsByPrimaryKey[key]; ok {
			item.Quantity += previous.Quantity
			item.Sum += previous.Sum
		}

		itemsByPrimaryKey[key] = item
	}

	items := make([]ShoppingReceiptItem, 0, len(itemsByPrimaryKey))
	for _, item := range itemsByPrimaryKey {
		items = append(items, item)
	}

	receipt.Receipt.Items = items
	receipt.OperationID = req.OperationID

	return &receipt, nil
}

func (c *Client[C]) GetTradingOperations(ctx context.Context, req TradingOperations) ([]TradingOperation, error) {
	if req.From.IsZero() {
		req.From = tradingTimeStart
	}

	if req.To.IsZero() {
		req.To = c.clock.Now()
	}

	resp, err := executeAuthorizedExchange[TradingOperationsResponse](ctx, c.client, req)
	if err != nil {
		return nil, err
	}

	for i := range resp.Items {
		resp.Items[i].Username = c.Username
	}

	return resp.Items, nil
}

func (c *Client[C]) GetPurchasedSecurities(ctx context.Context, req PurchasedSecurities) ([]PurchasedSecurity, error) {
	resp, err := executeAuthorizedExchange[purchasedSecuritiesResponse](ctx, c.client, req)
	if err != nil {
		return nil, err
	}

	now := c.clock.Now()
	for i := range resp.Data {
		resp.Data[i].Time = now
	}

	return resp.Data, nil
}

const maxCandleInterval = 12 * 30 * 24 * time.Hour // 1 year

func (c *Client[C]) GetCandles(ctx context.Context, req Candles) ([]Candle, error) {
	if req.From.IsZero() {
		req.From = tradingTimeStart
	}

	if req.To.IsZero() {
		req.To = c.clock.Now()
	}

	var candles []Candle
	for {
		if !req.From.Before(req.To) {
			break
		}

		partialReq := req
		partialReq.To = partialReq.From.Add(maxCandleInterval)
		if partialReq.To.After(req.To) {
			partialReq.To = req.To
		}

		partial, err := executeAuthorizedExchange[candlesResponse](ctx, c.client, partialReq)
		if err != nil {
			return nil, err
		}

		for i := range partial.Candles {
			partial.Candles[i].Ticker = req.Ticker
		}

		candles = append(candles, partial.Candles...)
		req.From = partialReq.To
	}

	return candles, nil
}

type client struct {
	Credential
	client    httpf.Client
	confirm   ConfirmFunc
	sessionID string
	commonsMu map[string]syncf.Locker
	mu        syncf.RWMutex
	cancel    func()
}

func (c *client) commonMu(operation string) syncf.Locker {
	locker, ok := c.commonsMu[operation]
	if !ok {
		return syncf.Unlock
	}

	return locker
}

func (c *client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	logf.Get(c).Resultf(req.Context(), logf.Debug, logf.Warn, "%s => %v", &httpf.RequestBuilder{Request: req}, err)
	return resp, err
}

func (c *client) resetSessionID(ctx context.Context) error {
	ctx, cancel := c.mu.Lock(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}

	defer cancel()
	c.sessionID = ""
	return nil
}

func (c *client) getSessionID(ctx context.Context) (string, error) {
	ctx, cancel := c.mu.RLock(ctx)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	defer cancel()
	return c.sessionID, nil
}

func executeCommonExchange[R any](ctx context.Context, client *client, exchange commonExchange[R]) (*commonResponse[R], error) {
	operation := exchange.operation()
	ctx, cancel := client.commonMu(operation).Lock(ctx)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	defer cancel()

	url := Host + "/api/common/v1/" + operation
	req := httpf.POST(url, httpf.FormValue(exchange)).
		Query("origin", "web,ib5,platform")
	if operation != "session" {
		sessionID, err := client.getSessionID(ctx)
		if err != nil {
			return nil, err
		}

		req.Query("sessionid", sessionID)
	}

	var resp commonResponse[R]
	if err := req.Exchange(ctx, client).
		CheckStatus(http.StatusOK).
		DecodeBody(flu.JSON(&resp)).
		Error(); err != nil {
		return nil, err
	}

	return &resp, resp.validate(exchange.resultCode())
}

func executeTradingExchange[R any](ctx context.Context, client *client, exchange tradingExchange[R]) (*tradingResponse[R], error) {
	url := Host + "/api/trading" + exchange.path()
	req := httpf.POST(url, flu.JSON(exchange)).
		Query("origin", "web,ib5,platform")

	sessionID, err := client.getSessionID(ctx)
	if err != nil {
		return nil, err
	}

	req.Query("sessionId", sessionID)

	var resp tradingResponse[R]
	if err := req.Exchange(ctx, client).
		CheckStatus(http.StatusOK, http.StatusAccepted).
		DecodeBody(flu.JSON(&resp)).
		Error(); err != nil {
		return nil, err
	}

	return &resp, resp.validate("Ok")
}

func (c *client) authorize(ctx context.Context) error {
	if c.confirm == nil {
		return errors.New("no confirm function is set")
	}

	ctx, cancel := c.mu.Lock(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	} else {
		defer cancel()
	}

	_ = c.Close()

	session, err := executeCommonExchange[string](ctx, c, session{})
	if err != nil {
		return errors.Wrap(err, "obtain new session ID")
	}

	c.sessionID = session.Payload

	phoneSignUp := phoneSignUp{Phone: c.Credential.Phone}
	phoneSignUpResponse, err := executeCommonExchange[Unit](ctx, c, phoneSignUp)
	if err != nil {
		return errors.Wrap(err, "phone sign up")
	}

	confirmation, err := c.confirm(ctx, c.Username)
	if err != nil {
		return errors.Wrap(err, "receive confirmation")
	}

	if _, err := executeCommonExchange[Unit](ctx, c, confirm{
		initialOperation:       phoneSignUp.operation(),
		initialOperationTicket: phoneSignUpResponse.OperationTicket,
		confirmationData: confirmationData{
			SMSBYID: strings.Trim(confirmation, " \n\t"),
		},
	}); err != nil {
		return errors.Wrap(err, "send confirmation")
	}

	if _, err := executeCommonExchange[Unit](ctx, c, passwordSignUp{Password: c.Credential.Password}); err != nil {
		return errors.Wrap(err, "password sign up")
	}

	if _, err := executeCommonExchange[Unit](ctx, c, levelUp{}); err != nil {
		return errors.Wrap(err, "level up")
	}

	return nil
}

func (c *client) ping() {
	c.cancel = syncf.GoSync(context.Background(), func(ctx context.Context) {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pong, err := executeCommonExchange[pong](ctx, c, ping{})
				logf.Get(c).Resultf(ctx, logf.Debug, logf.Warn, "ping => %+v (%v)", pong, err)
				if err != nil || pong.Payload.AccessLevel != "CLIENT" {
					_ = c.resetSessionID(ctx)
					return
				}
			}
		}
	})
}

var retryContextKey = "tinkoff.retry"

func executeAuthorizedExchange[R any](ctx context.Context, client *client, exchange exchange[R]) (R, error) {
	var zero R
	ctx, cancel := client.mu.Lock(ctx)
	if ctx.Err() != nil {
		return zero, ctx.Err()
	} else {
		defer cancel()
	}

	var err error
	if client.sessionID != "" {
		var payload R
		switch exchange := exchange.(type) {
		case commonExchange[R]:
			var resp *commonResponse[R]
			resp, err = executeCommonExchange[R](ctx, client, exchange)
			if err == nil {
				payload = resp.Payload
			}

		case tradingExchange[R]:
			var resp *tradingResponse[R]
			resp, err = executeTradingExchange[R](ctx, client, exchange)
			if err == nil {
				err = json.Unmarshal(resp.Payload, &payload)
			}

		default:
			return payload, errors.Errorf("unsupported %T exchange", exchange)
		}

		if !errors.Is(err, ErrInsufficientPrivileges) {
			return payload, err
		}

		if retry, _ := ctx.Value(retryContextKey).(bool); retry {
			return payload, err
		}

		ctx = context.WithValue(ctx, retryContextKey, true)
		client.sessionID = ""
	}

	if err := client.authorize(ctx); err != nil {
		_ = client.resetSessionID(ctx)
		return zero, errors.Wrap(err, "authorize")
	}

	client.ping()
	return executeAuthorizedExchange[R](ctx, client, exchange)
}

func (c *client) Close() error {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}

	return nil
}
