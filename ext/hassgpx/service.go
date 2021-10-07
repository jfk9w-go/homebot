package hassgpx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/homebot/core"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
)

type Service struct {
	flu.Clock
	Storage
	UserIDs      map[telegram.ID]string
	LastDays     int
	MaxSpeed     float64
	MoveInterval time.Duration
}

func (s *Service) Gate() core.Gate {
	userIDs := make(map[telegram.ID]bool, len(s.UserIDs))
	for userID := range s.UserIDs {
		userIDs[userID] = true
	}

	return core.Gate{UserIDs: userIDs}
}

func (s *Service) Get_GPX_track(ctx context.Context, client telegram.Client, cmd *telegram.Command) error {
	entityID := s.UserIDs[cmd.User.ID]
	since := s.Now().Add(-time.Duration(s.LastDays) * 24 * time.Hour)
	since = time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, time.UTC)
	waypoints, err := s.GetLastTrack(ctx, entityID, since, s.MaxSpeed, s.MoveInterval)
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
	_, err = client.Send(ctx, cmd.Chat.ID,
		&telegram.Media{
			Type:     telegram.Document,
			Input:    buffer,
			Filename: filename},
		nil)
	if err != nil {
		return errors.Wrap(err, "send gpx track")
	}

	return cmd.Reply(ctx, client, "OK")
}
