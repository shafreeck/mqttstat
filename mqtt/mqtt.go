package mqtt

import (
	"crypto/tls"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

type ClientConfig struct {
	Username     string
	Password     string
	ClientID     string
	CleanSession bool
	WillMessage  bool

	RecvHandler MessageHandler
	TLSConfig   *tls.Config
}

//ACK is the ack message of all control packets
type ACK struct {
	Err      error
	PacketID int
}

//MessageHandler is a callback to process the received message
type MessageHandler func(topic string, message []byte, qos int) error

//Client talks with server
type Client struct {
	conn   net.Conn
	hook   Hook
	cfg    *ClientConfig
	tracer Tracer
}

func NewClient(cfg *ClientConfig) *Client {
	c := new(Client)
	c.cfg = cfg
	c.tracer = defaultTracer
	return c
}

func (c *Client) SetDeadline(t time.Time) error {
	return nil
}

func (c *Client) Dial(url string, dialer ...net.Dialer) error {
	var d net.Dialer
	if len(dialer) > 0 {
		d = dialer[0]
	}

	const (
		tcpScheme = "tcp://"
		tlsScheme = "tls://"
		schemeLen = len(tcpScheme)
	)

	var scheme, host, port, addr string
	var err error
	switch {
	case strings.HasPrefix(url, tcpScheme):
		scheme = tcpScheme
		host, port, err = net.SplitHostPort(url[schemeLen:])
	case strings.HasPrefix(url, tlsScheme):
		scheme = tlsScheme
		host, port, err = net.SplitHostPort(url[schemeLen:])
	default:
		scheme = tcpScheme
		host, port, err = net.SplitHostPort(url)
	}

	if err != nil {
		return err
	}

	addr = net.JoinHostPort(host, port)
	if net.ParseIP(host) == nil {
		if c.tracer != nil {
			c.tracer.AddPoint(TraceDNSLookup, time.Now())
		}
		addrs, err := net.LookupHost(host)
		if err != nil {
			return err
		}
		addr = net.JoinHostPort(addrs[0], port)
	}

	if c.tracer != nil {
		c.tracer.AddPoint(TraceTCPDial, time.Now())
	}
	tcpConn, err := d.Dial("tcp", addr)
	if err != nil {
		return err
	}
	c.conn = tcpConn

	if scheme == tlsScheme {
		tlsConn := tls.Client(tcpConn, c.cfg.TLSConfig)
		c.tracer.AddPoint(TraceTLSDial, time.Now())
		if err := tlsConn.Handshake(); err != nil {
			return err
		}
		c.conn = tlsConn
	}

	//Send MQTT Connect packet
	cp := &packets.ConnectPacket{FixedHeader: packets.FixedHeader{MessageType: packets.Connect}}
	cp.Username = c.cfg.Username
	cp.Password = []byte(c.cfg.Password)
	cp.ProtocolVersion = 4
	cp.ProtocolName = "MQTT"
	cp.CleanSession = c.cfg.CleanSession
	cp.ClientIdentifier = c.cfg.ClientID

	cp.UsernameFlag = cp.Username != ""
	cp.PasswordFlag = len(cp.Password) > 0

	c.tracer.AddPoint(TraceConnect, time.Now())
	if err := cp.Write(c.conn); err != nil {
		return err
	}

	cap, err := packets.ReadPacket(c.conn)
	if err != nil {
		return err
	}

	//Panic here if the ack is invalid
	ack := cap.(*packets.ConnackPacket)
	if ack.ReturnCode != 0 {
		return errors.New("MQTT connect failed")
	}
	c.tracer.AddPoint(TraceConnack, time.Now())
	return nil
}

func (c *Client) Subscribe(topics []string, qoss []int) error {
	return nil
}

func (c *Client) Publish(topic string, message []byte, qos int) (chan ACK, error) {
	return nil, nil
}

func (c *Client) Disconnect() error {
	return nil
}

func (c *Client) TracePoints() []*TracePoint {
	return c.tracer.Points()
}
