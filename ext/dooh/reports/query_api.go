package reports

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jfk9w-go/flu"
	fluhttp "github.com/jfk9w-go/flu/http"
)

var QueryApiReportURL = "https://query-api.adhigh.net/api/v2/reports"

type QueryApiRequest struct {
	DatasetName string
	Start       time.Time
	End         time.Time
	Values      []string
	Timezone    string
	Keys        []string
	TotalRow    bool
}

func (r *QueryApiRequest) queryParams() url.Values {
	values := make(url.Values)
	values.Set("dataset_name", r.DatasetName)
	values.Set("start", r.Start.Format("2006-01-02"))
	values.Set("end", r.End.Add(-24*time.Hour).Format("2006-01-02"))
	values.Set("values", strings.Join(r.Values, ","))
	if r.Timezone != "" {
		values.Set("timezone", r.Timezone)
	}

	if len(r.Keys) > 0 {
		values.Set("keys", strings.Join(r.Keys, ","))
	}

	if r.TotalRow {
		values.Set("totalrow", "1")
	}

	return values
}

type QueryApiReport struct {
	Fields []string   `json:"fields"`
	Data   [][]string `json:"data"`
}

type QueryApiClient fluhttp.Client

func NewQueryApiClient(token string) *QueryApiClient {
	return (*QueryApiClient)(fluhttp.NewTransport().
		ResponseHeaderTimeout(10*time.Minute).
		NewClient().
		SetHeader("X-Auth-Token", token).
		AcceptStatus(http.StatusOK))
}

func (c *QueryApiClient) Unmask() *fluhttp.Client {
	return (*fluhttp.Client)(c)
}

func (c *QueryApiClient) GetReport(ctx context.Context, req *QueryApiRequest) (*QueryApiReport, error) {
	report := new(QueryApiReport)
	return report, c.Unmask().GET(QueryApiReportURL).
		QueryParams(req.queryParams()).
		QueryParam("format", "json").
		Context(ctx).
		Execute().
		DecodeBody(flu.JSON(report)).
		Error
}
