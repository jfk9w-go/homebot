package hassgpx

import (
	"context"
	_ "embed"
	"time"

	"github.com/jfk9w-go/flu/apfel"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

//go:embed ddl/gps.sql
var gpsDDL string

type Storage[C Context] struct {
	db *gorm.DB
}

func (s Storage[C]) String() string {
	return "hassgpx.storage"
}

func (s *Storage[C]) Include(ctx context.Context, app apfel.MixinApp[C]) error {
	config := app.Config().HassGPXConfig()
	if !config.Enabled {
		return apfel.ErrDisabled
	}

	gorm := &apfel.GormDB[C]{Config: config.DB}
	if err := app.Use(ctx, gorm, false); err != nil {
		return err
	}

	db := gorm.DB()
	if err := db.WithContext(ctx).Exec(gpsDDL).Error; err != nil {
		return errors.Wrap(err, "create gps view")
	}

	s.db = db
	return nil
}

func (s *Storage[C]) GetLastTrack(ctx context.Context, entityID string, since time.Time, maxSpeed float64, moveInterval time.Duration) ([]Waypoint, error) {
	rows := make([]Waypoint, 0)
	return rows, s.db.WithContext(ctx).Raw( /* language=SQL */ `
	select s1.time, s1.latitude, s1.longitude
	from gps s1 left join gps s2 on s1.old_state_id = s2.state_id
	where s1.entity_id like ?
		and abs(extract(epoch from s1.time - s2.time)) < ?
		and sqrt(pow(s1.latitude - s2.latitude, 2) + pow(s1.longitude - s2.longitude, 2)) / abs(extract(epoch from s1.time - s2.time)) * 111 * 3600 < ?
		and s1.time >= ?
	order by 1 asc`, entityID, moveInterval.Seconds(), maxSpeed, since).
		Scan(&rows).
		Error
}
