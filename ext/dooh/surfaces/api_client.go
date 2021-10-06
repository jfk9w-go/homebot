package surfaces

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jfk9w-go/flu"
	fluhttp "github.com/jfk9w-go/flu/http"
	"github.com/pkg/errors"
)

var BaseURL = "https://dooh-api.adhigh.net/api/v1"

type ApiClient struct {
	httpClient      *fluhttp.Client
	email, password string
}

func NewApiClient(email, password string) *ApiClient {
	httpClient := fluhttp.NewClient(nil)
	return &ApiClient{
		httpClient: httpClient,
		email:      email,
		password:   password,
	}
}

func (c *ApiClient) newRequest(ctx context.Context, method, path string) (*fluhttp.Request, error) {
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

func (c *ApiClient) Surfaces(ctx context.Context) ([]Surface, error) {
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
