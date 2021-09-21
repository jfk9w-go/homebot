package app

import (
	"context"
	"io"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/metrics"
	"github.com/jfk9w-go/telegram-bot-api"
	"gorm.io/gorm"
)

type Interface interface {
	flu.Clock
	GetConfig(value interface{}) error
	GetMetricsRegistry(ctx context.Context) (metrics.Registry, error)
	GetDatabase(conn string) (*gorm.DB, error)
	Manage(service io.Closer)
}

type Extension interface {
	Key() string
	Apply(ctx context.Context, app Interface) (telegram.CommandListener, error)
}
