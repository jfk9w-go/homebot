package hassgpx

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type SQLStorage gorm.DB

func (s *SQLStorage) Unmask() *gorm.DB {
	return (*gorm.DB)(s)
}

func (s *SQLStorage) GetLastTrack(ctx context.Context, entityID string, since time.Time) ([]Waypoint, error) {
	rows := make([]Waypoint, 0)
	return rows, s.Unmask().WithContext(ctx).Raw( /* language=SQL */ `
		select jsonb_extract_path(attributes::jsonb, 'latitude') as latitude,
			   jsonb_extract_path(attributes::jsonb, 'longitude') as longitude,
			   jsonb_extract_path(attributes::jsonb, 'altitude') as elevation,
			   created as time
		from states
		where entity_id = ? and created >= ?
		order by time`, entityID, since).
		Scan(&rows).
		Error
}
