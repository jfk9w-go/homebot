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
	GetDatabase(driver, conn string) (*gorm.DB, error)
	GetBot(ctx context.Context) (*telegram.Bot, error)
	Manage(service io.Closer)
}

type Extension interface {
	ID() string
	Apply(ctx context.Context, app Interface) (interface{}, error)
}
