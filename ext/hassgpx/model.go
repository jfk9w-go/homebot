package hassgpx

import (
	"context"
	"encoding/xml"
	"time"
)

type GPX struct {
	XMLName xml.Name `xml:"gpx"`

	XMLNS          string `xml:"xmlns,attr"`
	Creator        string `xml:"creator,attr"`
	Version        string `xml:"version,attr"`
	XSI            string `xml:"xmlns:xsi,attr"`
	SchemaLocation string `xml:"xsi:schemaLocation,attr"`

	Metadata Metadata `xml:"metadata"`
	Track    Track    `xml:"trk"`
}

type Metadata struct {
	Name string `xml:"name"`
	Desc string `xml:"desc"`
}

type Track struct {
	Metadata
	Segment TrackSegment `xml:"trkseg"`
}

type TrackSegment struct {
	Waypoints []Waypoint `xml:"trkpt"`
}

type Waypoint struct {
	Latitude  float64   `xml:"lat,attr"`
	Longitude float64   `xml:"lon,attr"`
	Elevation float64   `xml:"ele"`
	Time      time.Time `xml:"time"`
}

type Storage interface {
	GetLastTrack(ctx context.Context, entityID string, since time.Time, maxSpeed float64) ([]Waypoint, error)
}
