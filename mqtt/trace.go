package mqtt

import (
	"time"
)

const (
	TraceDNSLookup = "DNSLookup"
	TraceTCPDial   = "TCPDial"
	TraceTLSDial   = "TLSDial"
	TraceConnect   = "Connect"
	TraceConnack   = "Connack"
	TraceSubscribe = "Subscribe"
	TraceSuback    = "Suback"
	TracePublish   = "Publish"
	TracePuback    = "Puback"
)

type Tracer interface {
	AddPoint(key string, t time.Time)
	Points() []*TracePoint
}

type TracePoint struct {
	Key  string
	Time time.Time
}

type tracer struct {
	points []*TracePoint
}

func (t *tracer) AddPoint(key string, tm time.Time) {
	t.points = append(t.points, &TracePoint{Key: key, Time: tm})
}

func (t *tracer) Points() []*TracePoint {
	return t.points
}

var defaultTracer Tracer

func init() {
	defaultTracer = &tracer{points: make([]*TracePoint, 0)}
}
