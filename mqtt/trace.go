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
	TraceMessage   = "Message"
	TracePing      = "Ping"
	TracePong      = "Pong"
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

func DefaultTracer() Tracer {
	return &tracer{points: make([]*TracePoint, 0)}
}
