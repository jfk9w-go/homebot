package dooh

import (
	"context"
	"fmt"
	"time"

	"github.com/jfk9w-go/flu"
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

type Service struct {
	Client         *Client
	TelegramClient telegram.Client
	File           flu.File
	ChatID         telegram.ID
	work           flu.WaitGroup
	cancel         func()
}

func (s *Service) RunInBackground(ctx context.Context, updateEvery time.Duration) error {
	if s.cancel != nil {
		return nil
	}

	if err := s.UpdateSurfaces(ctx); err != nil {
		return err
	}

	s.cancel = s.work.Go(ctx, func(ctx context.Context) {
		ticker := time.NewTicker(updateEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.UpdateSurfaces(ctx); err != nil {
					return
				}
			}
		}
	})

	return nil
}

func (s *Service) Close() error {
	if s.cancel != nil {
		s.cancel()
		s.work.Wait()
	}

	return nil
}

func (s *Service) UpdateSurfaces(ctx context.Context) error {
	writer := s.newHTMLWriter(ctx)
	if err := s.doUpdateSurfaces(ctx, writer); err != nil {
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

func (s *Service) doUpdateSurfaces(ctx context.Context, writer *html.Writer) error {
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
		name := fmt.Sprintf("[%s %s] %s",
			surface.Attributes.Network,
			surface.Attributes.SurfaceID,
			surface.Attributes.Name)
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

		writer.Text(" ").Link(name, url).Text("\n")
		updated.Surfaces[surface.ID] = surface
	}

	if err := flu.EncodeTo(flu.Gob(updated), s.File); err != nil {
		return errors.Wrap(err, "save surfaces")
	}

	logrus.WithField("service", "dooh").Info("updated surfaces")
	return nil
}

func (s *Service) newHTMLWriter(ctx context.Context) *html.Writer {
	return &html.Writer{
		Context: ctx,
		Out: &output.Paged{
			Receiver: &receiver.Chat{
				Sender:    s.TelegramClient,
				ID:        s.ChatID,
				ParseMode: telegram.HTML,
			},
			PageSize: telegram.MaxMessageSize,
		},
	}
}
