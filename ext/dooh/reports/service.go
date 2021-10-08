package reports

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/homebot/core"
	"github.com/jfk9w-go/homebot/ext/dooh"
	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var Values = []string{"bids", "shows"}

type Data map[string]map[string]float64

type Service struct {
	*dooh.Service
	Clock          flu.Clock
	QueryApiClient *QueryApiClient
	Clickhouse     *gorm.DB
	LastDays       int
	Thresholds     Thresholds
	cron           *cron.Cron
}

func (s *Service) RunInBackground(ctx context.Context, spec string) error {
	if s.cron != nil {
		return nil
	}

	s.cron = cron.New()
	if _, err := s.cron.AddFunc(spec, func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		if err := s.run(ctx, s.LastDays); err != nil {
			logrus.Warnf("check reports: %s", err)
		}
	}); err != nil {
		return errors.Wrap(err, "failed to schedule cron task")
	}

	s.cron.Start()
	return nil
}

func (s *Service) Close() error {
	if s.cron != nil {
		select {
		case <-time.After(10 * time.Second):
			return errors.New("timeout on exit")
		case <-s.cron.Stop().Done():
			return nil
		}
	}

	return nil
}

func (s *Service) Check_reports(ctx context.Context, tgclient telegram.Client, cmd *telegram.Command) error {
	lastDays := s.LastDays
	if len(cmd.Args) > 0 {
		var err error
		lastDays, err = strconv.Atoi(cmd.Args[0])
		if err != nil || lastDays < 1 {
			return errors.Errorf("invalid last days: %s", cmd.Args[0])
		}
	}

	if err := s.run(ctx, lastDays); err != nil {
		return errors.Wrap(err, "check reports")
	}

	return nil
}

func (s *Service) run(ctx context.Context, lastDays int) error {
	report := core.NewJobReport()
	report.Title(fmt.Sprintf("Сравнение отчетов ГПМ vs GI (%d дней)", lastDays))
	if err := s.runWith(ctx, lastDays, report); err != nil {
		if flu.IsContextRelated(err) {
			return err
		}

		report.Error("Run check", err.Error())
	}

	return report.DumpTo(ctx, s.NewOutput())
}

func (s *Service) runWith(ctx context.Context, lastDays int, report *core.JobReport) error {
	end := truncateDay(s.Clock.Now())
	start := truncateDay(end.Add(-time.Duration(lastDays*24) * time.Hour))
	clickhouse, err := s.getClickhouseData(ctx, start, end)
	if err != nil {
		return errors.Wrap(err, "get clickhouse data")
	}

	queryApi, err := s.getQueryApiData(ctx, start, end)
	if err != nil {
		return errors.Wrap(err, "get query-api data")
	}

	invalidDays := 0
	for date := start; date.Before(end); date = date.Add(24 * time.Hour) {
		day := date.Format("2006-01-02")
		comparison := compare(clickhouse[day], queryApi[day])
		if comparison.breaks(s.Thresholds.Error) {
			report.Error(day, comparison.String())
			invalidDays++
		} else if comparison.breaks(s.Thresholds.Warn) {
			report.Warn(day, comparison.String())
			invalidDays++
		} else if comparison.breaks(s.Thresholds.Info) {
			report.Info(day, comparison.String())
			invalidDays++
		}
	}

	if invalidDays == 0 {
		report.Title("OK")
	}

	logrus.Infof("check reports: ok")
	return nil
}

func (s *Service) getClickhouseData(ctx context.Context, start, end time.Time) (Data, error) {
	rows := make([]map[string]interface{}, 0)
	if err := s.Clickhouse.WithContext(ctx).Raw( /* language=SQL */ `
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

func (s *Service) getQueryApiData(ctx context.Context, start, end time.Time) (Data, error) {
	response, err := s.QueryApiClient.GetReport(ctx, &QueryApiRequest{
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
