package dooh

import (
	"context"
	"fmt"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/homebot/core"
	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/html"
	"github.com/jfk9w-go/telegram-bot-api/ext/output"
	"github.com/jfk9w-go/telegram-bot-api/ext/receiver"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type SurfaceDictionary struct {
	Surfaces map[string]Surface
}

type CommandListener struct {
	Client         *Client
	TelegramClient telegram.Client
	File           flu.File
	ChatID         telegram.ID
	ControlButtons *core.ControlButtons
	mu             flu.Mutex
	work           flu.WaitGroup
	cancel         func()
}

func (l *CommandListener) Allow(chatID, userID telegram.ID) bool {
	return l.ChatID == chatID
}

func (l *CommandListener) RunInBackground(ctx context.Context, updateEvery time.Duration) error {
	if l.cancel != nil {
		return nil
	}

	if err := l.updateSurfaces(ctx); err != nil {
		return err
	}

	l.cancel = l.work.Go(ctx, func(ctx context.Context) {
		ticker := time.NewTicker(updateEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := l.updateSurfaces(ctx); err != nil {
					return
				}
			}
		}
	})

	return nil
}

func (l *CommandListener) Close() error {
	if l.cancel != nil {
		l.cancel()
		l.work.Wait()
	}

	return nil
}

func (l *CommandListener) Update_surfaces(ctx context.Context, tgclient telegram.Client, cmd *telegram.Command) error {
	if err := l.updateSurfaces(ctx); err != nil {
		return errors.Wrap(err, "update surfaces")
	}

	return cmd.Reply(ctx, tgclient, "OK")
}

func (l *CommandListener) updateSurfaces(ctx context.Context) error {
	writer := l.newHTMLWriter(ctx)
	if err := l.doUpdateSurfaces(ctx, writer); err != nil {
		if flu.IsContextRelated(err) {
			return err
		}

		writer.Text(err.Error())
	}

	if err := writer.Flush(); err != nil {
		if flu.IsContextRelated(err) {
			return err
		}

		logrus.WithField("service", "dooh").Errorf("flush html writer: %s", err)
	}

	return nil
}

func (s *CommandListener) doUpdateSurfaces(ctx context.Context, writer *html.Writer) error {
	defer s.mu.Lock().Unlock()
	surfaces, err := s.Client.Surfaces(ctx)
	if err != nil {
		return errors.Wrap(err, "get surfaces")
	}

	existing := new(SurfaceDictionary)
	if ok, err := s.File.Exists(); ok {
		if err := flu.DecodeFrom(s.File, flu.Gob(existing)); err != nil {
			return errors.Wrap(err, "decode file")
		}
	} else if err != nil {
		return errors.Wrap(err, "exists")
	}

	updated := &SurfaceDictionary{
		Surfaces: make(map[string]Surface),
	}

	for _, surface := range surfaces {
		updated.Surfaces[surface.ID] = surface
		externalID := fmt.Sprintf("[%s %s]",
			surface.Attributes.Network,
			surface.Attributes.SurfaceID)
		url := ResourceURL("surfaces", surface.ID)
		if entry, ok := existing.Surfaces[surface.ID]; ok {
			if entry.Attributes.UpdatedAt != surface.Attributes.UpdatedAt {
				if entry.Attributes.DeletedAt == nil && surface.Attributes.DeletedAt != nil {
					writer.Bold("DELETED")
				} else if entry.Attributes.DeletedAt != nil && surface.Attributes.DeletedAt == nil {
					writer.Bold("RESTORED")
				} else {
					writer.Bold("UPDATED")
				}
			} else {
				continue
			}
		} else {
			writer.Bold("ADDED")
		}

		writer.Text(" ").Link(externalID, url).Text(" " + surface.Attributes.Name).Text("\n")
	}

	if err := writer.Flush(); err != nil {
		return errors.Wrap(err, "flush html writer")
	}

	if err := flu.EncodeTo(flu.Gob(updated), s.File); err != nil {
		return errors.Wrap(err, "save surfaces")
	}

	logrus.WithField("service", "dooh").Info("updated surfaces")
	return nil
}

func (l *CommandListener) newHTMLWriter(ctx context.Context) *html.Writer {
	return &html.Writer{
		Context: ctx,
		Out: &output.Paged{
			Receiver: &receiver.Chat{
				Sender:      l.TelegramClient,
				ID:          l.ChatID,
				ParseMode:   telegram.HTML,
				ReplyMarkup: l.ControlButtons.Keyboard(l.ChatID, 0),
			},
			PageSize: telegram.MaxMessageSize,
		},
	}
}
