package hassgpx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"homebot/common"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/apfel"
	"github.com/jfk9w-go/flu/colf"
	"github.com/jfk9w-go/flu/syncf"
	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/tapp"
	"github.com/pkg/errors"
)

type (
	Config struct {
		Enabled      bool                   `yaml:"enabled,omitempty" doc:"Enables the service and bot command."`
		DB           apfel.GormConfig       `yaml:"db" doc:"Home Assistant database connection settings. This is used for collecting tracking data from Home Assistant."`
		MaxSpeed     float64                `yaml:"maxSpeed,omitempty" doc:"Maximum speed to be considered \"in track\". This is a really rough approximation via coordinate and time change between two consecutive tracking points,\nand is used to distinguish between using bicycle and other vehicles (like city trains when you forget to turn off tracking – again, REALLY rough approximation)." default:"55"`
		LastDays     int                    `yaml:"lastDays,omitempty" doc:"Number of full past days to detect bicycle tracks over. 0 means 'today'." default:"0"`
		MoveInterval flu.Duration           `yaml:"moveInterval,omitempty" doc:"If two consecutive tracking points are within this time interval, they are considered to be 'in track'.\nYou need to turn on frequent location updates in Home Assistant app on your phone in order to start 'tracking'." default:"1m" format:"duration"`
		Users        map[telegram.ID]string `yaml:"users" doc:"Telegram user ID to Home Assistant device name filter mapping. Only users with IDs from this dictionary will be allowed to execute /get_gpx_track."`
	}

	Context interface{ HassGPXConfig() Config }
)

type Mixin[C Context] struct {
	clock        syncf.Clock
	storage      Storage[C]
	users        map[telegram.ID]string
	maxSpeed     float64
	lastDays     int
	moveInterval time.Duration
}

func (m *Mixin[C]) String() string {
	return "hassgpx"
}

func (m *Mixin[C]) Include(ctx context.Context, app apfel.MixinApp[C]) error {
	config := app.Config().HassGPXConfig()
	if !config.Enabled {
		return apfel.ErrDisabled
	}

	var db Storage[C]
	if err := app.Use(ctx, &db, false); err != nil {
		return err
	}

	m.clock = app
	m.storage = db
	m.users = config.Users
	m.maxSpeed = config.MaxSpeed
	m.lastDays = config.LastDays
	m.moveInterval = config.MoveInterval.Value

	return nil
}

func (m *Mixin[C]) CommandScope() tapp.CommandScope {
	userIDs := make(colf.Set[telegram.ID], len(m.users))
	for userID := range m.users {
		userIDs.Add(userID)
	}

	return tapp.CommandScope{UserIDs: userIDs}
}

//goland:noinspection GoSnakeCaseUsage
func (m *Mixin[C]) Get_GPX_track(ctx context.Context, client telegram.Client, cmd *telegram.Command) error {
	entityID := m.users[cmd.User.ID]
	since := m.clock.Now().Add(-time.Duration(m.lastDays) * 24 * time.Hour)
	since = common.TrimDate(since)
	waypoints, err := m.storage.GetLastTrack(ctx, entityID, since, m.maxSpeed, m.moveInterval)
	if err != nil {
		return errors.Wrap(err, "get last track")
	}

	if len(waypoints) == 0 {
		return errors.New("no recent tracks")
	}

	gpx := &GPX{
		XMLNS:          "https://www.topografix.com/GPX/1/1",
		Creator:        "github.com/jfk9w-go/homebot",
		Version:        "1.1",
		XSI:            "https://www.w3.org/2001/XMLSchema-instance",
		SchemaLocation: "https://www.topografix.com/GPX/1/1 https://www.topografix.com/GPX/1/1/gpx.xsd",
		Metadata: Metadata{
			Name: waypoints[0].Time.String(),
			Desc: fmt.Sprintf("%s – %s", waypoints[0].Time, waypoints[len(waypoints)-1].Time),
		},
		Track: Track{
			Segment: TrackSegment{
				Waypoints: waypoints,
			},
		},
	}

	buffer := new(flu.ByteBuffer)
	if err := flu.EncodeTo(flu.XML(gpx), buffer); err != nil {
		return errors.Wrap(err, "encode value")
	}

	filename := strings.Replace(waypoints[0].Time.String(), ":", "_", -1) + ".gpx"
	if _, err := client.Send(ctx, cmd.Chat.ID,
		&telegram.Media{
			Type:     telegram.Document,
			Input:    buffer,
			Filename: filename,
		}, nil,
	); err != nil {
		return errors.Wrap(err, "send gpx track")
	}

	return nil
}
