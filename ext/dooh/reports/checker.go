package reports

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jasonlvhit/gocron"
	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/homebot/core"
	"github.com/jfk9w-go/homebot/ext/dooh"
	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

var Values = []string{"bids", "shows"}

type Data map[string]map[string]float64

type Checker struct {
	*dooh.Service
	Clock          flu.Clock
	QueryApiClient *QueryApiClient
	Clickhouse     *gorm.DB
	LastDays       int
	Thresholds     Thresholds
	scheduler      *gocron.Scheduler
}

func (c *Checker) RunInBackground(ctx context.Context, at string) error {
	if c.scheduler != nil {
		return nil
	}

	c.scheduler = gocron.NewScheduler()
	c.scheduler.Start()
	return c.scheduler.Every(1).Day().At(at).Do(c.run, context.Background())
}

func (c *Checker) Close() error {
	if c.scheduler != nil {
		c.scheduler.Clear()
	}

	return nil
}

func (c *Checker) Check_reports(ctx context.Context, tgclient telegram.Client, cmd *telegram.Command) error {
	if err := c.run(ctx); err != nil {
		return errors.Wrap(err, "update surfaces")
	}

	return cmd.Reply(ctx, tgclient, "OK")
}

func (c *Checker) run(ctx context.Context) error {
	report := core.NewJobReport()
	report.Title("Сравнение отчетов ГПМ vs GI")
	if err := c.runWith(ctx, report); err != nil {
		if flu.IsContextRelated(err) {
			return err
		}

		report.Error("Run check", err.Error())
	}

	return report.DumpTo(ctx, c.NewOutput())
}

func (c *Checker) runWith(ctx context.Context, report *core.JobReport) error {
	end := truncateDay(c.Clock.Now())
	start := truncateDay(end.Add(-time.Duration(c.LastDays*24) * time.Hour))
	clickhouse, err := c.getClickhouseData(ctx, start, end)
	if err != nil {
		return errors.Wrap(err, "get clickhouse data")
	}

	queryApi, err := c.getQueryApiData(ctx, start, end)
	if err != nil {
		return errors.Wrap(err, "get query-api data")
	}

	for date := start; date.Before(end); date = date.Add(24 * time.Hour) {
		day := date.Format("2006-01-02")
		comparison := compare(clickhouse[day], queryApi[day])
		if comparison.breaks(c.Thresholds.Error) {
			report.Error(day, comparison.String())
		} else if comparison.breaks(c.Thresholds.Warn) {
			report.Warn(day, comparison.String())
		} else if comparison.breaks(c.Thresholds.Info) {
			report.Info(day, comparison.String())
		}
	}

	return nil
}

type comparison map[string][3]float64

func compare(a, b map[string]float64) comparison {
	c := make(map[string][3]float64)
	for _, field := range Values {
		a := a[field]
		b := b[field]
		if a == 0 && b != 0 {
			c[field] = [3]float64{a, b, 1}
		} else if a != 0 {
			c[field] = [3]float64{a, b, b/a - 1}
		}
	}

	return c
}

func (c comparison) breaks(threshold float64) bool {
	for _, entry := range c {
		if math.Abs(entry[2]) > threshold {
			return true
		}
	}

	return false
}

func (c comparison) String() string {
	var b strings.Builder
	for field, entry := range c {
		diff := entry[2]
		sign := ""
		if diff > 0 {
			sign = "+"
		} else if diff < 0 {
			sign = "-"
		} else {
			continue
		}

		b.WriteString(fmt.Sprintf("%s: %.0f / %.0f / %s%.5f%s\n", field, entry[0], entry[1], sign, math.Abs(diff), "%"))
	}

	return strings.Trim(b.String(), "\n")
}

func (c *Checker) getClickhouseData(ctx context.Context, start, end time.Time) (Data, error) {
	rows := make([]map[string]interface{}, 0)
	if err := c.Clickhouse.WithContext(ctx).Raw( /* language=SQL */ `
		select toString(toDate(fromUnixTimestamp64Milli(bid.timestamp))) as day,
			   toFloat64(count(1)) as bids,
			   toFloat64(count(clearance_price)) as shows
		from ods_bid bid left join ods_burl burl on bid.bid_id = burl.bid_id
		where bid.timestamp >= ? and bid.timestamp < ?
		group by day
		order by day`, start.UnixMilli(), end.UnixMilli()).
		Scan(&rows).
		Error; err != nil {
		return nil, errors.Wrap(err, "get clickhouse report")
	}

	report := make(Data)
	for _, row := range rows {
		day := row["day"].(string)
		values := make(map[string]float64, len(row)-1)
		for key, value := range row {
			if key == "day" {
				continue
			}

			values[key] = value.(float64)
		}

		report[day] = values
	}

	return report, nil
}

func (c *Checker) getQueryApiData(ctx context.Context, start, end time.Time) (Data, error) {
	response, err := c.QueryApiClient.GetReport(ctx, &QueryApiRequest{
		DatasetName: "dooh_report_v2",
		Start:       start,
		End:         end,
		Values:      Values,
		Timezone:    "UTC",
		Keys:        []string{"day"},
	})

	if err != nil {
		return nil, errors.Wrap(err, "get query api report")
	}

	report := make(Data)
	for _, row := range response.Data {
		var day string
		values := make(map[string]float64)
		for i, field := range response.Fields {
			if field == "day" {
				day = row[i]
				continue
			}

			value, err := strconv.ParseFloat(row[i], 64)
			if err != nil {
				return nil, errors.Wrapf(err, "parse value: %s (%s)", row[i], err)
			}

			values[field] = value
		}

		report[day] = values
	}

	return report, nil
}

func truncateDay(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
}
