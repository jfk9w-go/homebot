package tinkoff

import (
	"context"
	"strconv"
	"strings"
	"time"

	"homebot/tinkoff/external"

	"github.com/jfk9w-go/flu"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext"
	"github.com/jfk9w-go/telegram-bot-api/ext/tapp"
	"github.com/pkg/errors"
)

type Context struct {
	Storage
}

type Service struct {
	*Context
	flu.Clock
	Credentials CredentialStore
	Executors   []Executor
}

func (s *Service) CommandScope() tapp.CommandScope {
	userIDs := make(map[telegram.ID]bool, len(s.Credentials))
	for userID := range s.Credentials {
		userIDs[userID] = true
	}

	return tapp.CommandScope{UserIDs: userIDs}
}

func (s *Service) Update_bank_statement(ctx context.Context, tgclient telegram.Client, cmd *telegram.Command) error {
	cred := s.Credentials[cmd.User.ID]
	client, err := external.Authorize(ctx, cred, func(ctx context.Context) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		m, err := tgclient.Ask(ctx, cmd.Chat.ID, &telegram.Text{Text: "Code:"}, nil)
		if err != nil {
			return "", err
		}

		return strings.Trim(m.Text, " \n"), nil
	})

	if err != nil {
		return err
	}

	reload := 60 * 24 * time.Hour
	if daysStr := cmd.Arg(0); daysStr != "" {
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return errors.Wrap(err, "first parameter must be empty or a number of days")
		}

		reload = time.Duration(days) * 24 * time.Hour
	}

	sync := &Sync{
		Context: s.Context,
		Client:  client,
		Now:     s.Now(),
		Reload:  reload,
		report:  ext.HTML(ctx, tgclient, cmd.Chat.ID),
	}

	for _, executor := range s.Executors {
		if err := sync.Run(ctx, executor); err != nil {
			return err
		}
	}

	return sync.Close()
}
