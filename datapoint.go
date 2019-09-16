package sds011

import (
	"fmt"
	"time"
)

// DataPoint denotes a set of data taken at a specific point in time
type DataPoint struct {
	TimeStamp time.Time
	PM25      float64
	PM10      float64
}

// String returns a well-formatted string for the data point, fulfilling the Stringer interface
func (p *DataPoint) String() string {
	return fmt.Sprintf("%s: %.1f (PM2.5), %.1f (PM10) μg / ㎥", p.TimeStamp.Format(time.RFC1123),
		p.PM25,
		p.PM10)
}
