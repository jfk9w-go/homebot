package hassgpx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jfk9w-go/flu"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"

	"github.com/jfk9w-go/homebot/core"
)

type CommandListener struct {
	flu.Clock
	Storage
	*core.ControlButtons
	Users    map[telegram.ID]string
	Lookback time.Duration
}

func (l *CommandListener) OnCommand(ctx context.Context, client telegram.Client, cmd *telegram.Command) error {
	entityID, ok := l.Users[cmd.User.ID]
	if !ok {
		return errors.New("unknown user")
	}

	since := l.Now().Add(-l.Lookback)
	waypoints, err := l.GetLastTrack(ctx, entityID, since)
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
		&telegram.SendOptions{
			ReplyMarkup: l.Keyboard()})
	return err
}
