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

func (s *SQLStorage) GetLastTrack(ctx context.Context, entityID string, since time.Time, maxSpeed float64) ([]Waypoint, error) {
	rows := make([]Waypoint, 0)
	return rows, s.Unmask().WithContext(ctx).Raw( /* language=SQL */ `
	select s1.time, s1.latitude, s1.longitude
	from gps s1 left join gps s2 on s1.old_state_id = s2.state_id
	where s1.entity_id like ?
		and abs(extract(epoch from s1.time - s2.time)) < 60
		and sqrt(pow(s1.latitude - s2.latitude, 2) + pow(s1.longitude - s2.longitude, 2)) / abs(extract(epoch from s1.time - s2.time)) * 111 * 3600 < ?
		and s1.time >= ?
	order by 1 asc`, entityID, maxSpeed, since).
		Scan(&rows).
		Error
}
