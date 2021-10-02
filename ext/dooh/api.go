package dooh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jfk9w-go/flu"
	fluhttp "github.com/jfk9w-go/flu/http"
	"github.com/pkg/errors"
)

var BaseURL = "https://dooh-api.adhigh.net/api/v1"

type EmailAuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type TokenResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
}

type Time time.Time

func (t *Time) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	var err error
	*(*time.Time)(t), err = time.Parse(time.RFC3339, str)
	return err
}

func (t *Time) GobDecode(data []byte) error {
	return flu.DecodeFrom(flu.Bytes(data), flu.Gob((*time.Time)(t)))
}

func (t Time) GobEncode() ([]byte, error) {
	buf := new(flu.ByteBuffer)
	if err := flu.EncodeTo(flu.Gob(time.Time(t)), buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (t Time) String() string {
	return time.Time(t).String()
}

type Surface struct {
	ID         string `json:"id"`
	Attributes struct {
		Name      string `json:"name"`
		Network   string `json:"network"`
		SurfaceID string `json:"surfaceId"`
		CreatedAt Time   `json:"createdAt"`
		UpdatedAt Time   `json:"updatedAt"`
		DeletedAt *Time  `json:"deletedAt"`
	}
}

type Client struct {
	httpClient      *fluhttp.Client
	email, password string
}

func NewClient(email, password string) *Client {
	httpClient := fluhttp.NewClient(nil)
	return &Client{
		httpClient: httpClient,
		email:      email,
		password:   password,
	}
}

func (c *Client) newRequest(ctx context.Context, method, path string) (*fluhttp.Request, error) {
	req := &EmailAuthRequest{
		Email:    c.email,
		Password: c.password,
	}

	resp := new(TokenResponse)
	if err := c.httpClient.POST(BaseURL + "/users/login").
		Context(ctx).
		BodyEncoder(flu.JSON(req)).
		Execute().
		CheckStatus(http.StatusOK).
		DecodeBody(flu.JSON(resp)).
		Error; err != nil {
		return nil, errors.Wrap(err, "authorize")
	}

	return c.httpClient.NewRequest(method, BaseURL+path).
		Context(ctx).
		Auth(fluhttp.Bearer(resp.Token)), nil
}

func (c *Client) Surfaces(ctx context.Context) ([]Surface, error) {
	var resp struct {
		Data []Surface `json:"data"`
	}

	request, err := c.newRequest(ctx, http.MethodGet, "/surfaces")
	if err != nil {
		return nil, errors.Wrap(err, "create request")
	}

	if err := request.
		QueryParam("filter[deleted]", "any").
		Execute().
		CheckStatus(http.StatusOK).
		DecodeBody(flu.JSON(&resp)).
		Error; err != nil {
		return nil, errors.Wrap(err, "execute request")
	}

	return resp.Data, nil
}

func ResourceURL(resourceType string, id string) string {
	return fmt.Sprintf("https://dooh-ui.adhigh.net/%s/%s/edit", resourceType, id)
}
