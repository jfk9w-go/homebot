package external

import "time"

var MoscowLocation *time.Location

func init() {
	var err error
	MoscowLocation, err = time.LoadLocation("Europe/Moscow")
	if err != nil {
		panic(err)
	}
}
