package showrss

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-querystring/query"
	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/apfel"
	"github.com/jfk9w-go/flu/httpf"
	"github.com/jfk9w-go/flu/logf"
	"github.com/pkg/errors"
)

var URLTemplate = "http://showrss.info/user/%s.rss"

type Options struct {
	Magnets bool   `url:"magnets,omitempty"`
	Quality string `url:"quality"`
}

type Response struct {
	Channel struct {
		Title string `xml:"title"`
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			InfoHash    string `xml:"http://showrss.info info_hash"`
		} `xml:"item"`
	} `xml:"channel"`
}

type Client[C any] struct {
	*client
}

func (c *Client[C]) Include(ctx context.Context, app apfel.MixinApp[C]) error {
	c.client = &client{
		client: new(http.Client),
	}

	return nil
}

type client struct {
	client httpf.Client
}

func (c *client) String() string {
	return "showrss.client"
}

func (c *client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	logf.Get(c).Resultf(req.Context(), logf.Trace, logf.Warn, "%s => %v", &httpf.RequestBuilder{Request: req}, err)
	return resp, err
}

func (c *client) GetFeed(ctx context.Context, userID string, options Options) (*Response, error) {
	if options.Quality == "" {
		options.Quality = "fhd"
	}

	values, err := query.Values(options)
	if err != nil {
		return nil, errors.Wrap(err, "encode values")
	}

	var resp Response
	return &resp, httpf.GET(fmt.Sprintf(URLTemplate, userID)).
		QueryValues(values).
		Exchange(ctx, c).
		CheckStatus(http.StatusOK).
		DecodeBody(flu.XML(&resp)).
		Error()
}
