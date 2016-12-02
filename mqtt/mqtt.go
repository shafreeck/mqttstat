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
	TLSConfig   tls.Config
	TCPConfig   TCPConfig
}

type TCPConfig struct {
	Linger    int
	NoDelay   bool
	SendBuf   int
	RecvBuf   int
	Keepalive bool
}

//ACK is the ack message of all control packets
type ACK struct {
	ControlPacket packets.ControlPacket
	PacketID      int
}

//MessageHandler is a callback to process the received message
type MessageHandler func(topic string, message []byte, qos int) error

//Client talks with server
type Client struct {
	conn   net.Conn
	cfg    *ClientConfig
	tracer Tracer

	errc chan error
	rr   map[uint16]chan ACK //request-reply mapping
	id   uint64
}

func NewClient(cfg *ClientConfig) *Client {
	c := new(Client)
	c.cfg = cfg
	c.tracer = DefaultTracer()
	c.errc = make(chan error, 1)
	c.rr = make(map[uint16]chan ACK)
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
	tcpc := tcpConn.(*net.TCPConn)
	tcpc.SetKeepAlive(c.cfg.TCPConfig.Keepalive)
	tcpc.SetLinger(c.cfg.TCPConfig.Linger)
	tcpc.SetNoDelay(c.cfg.TCPConfig.NoDelay)
	if c.cfg.TCPConfig.RecvBuf > 0 {
		tcpc.SetReadBuffer(c.cfg.TCPConfig.RecvBuf)
	}
	if c.cfg.TCPConfig.SendBuf > 0 {
		tcpc.SetWriteBuffer(c.cfg.TCPConfig.SendBuf)
	}
	c.conn = tcpc

	if scheme == tlsScheme {
		tlsConn := tls.Client(tcpConn, &c.cfg.TLSConfig)
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

	go c.recvHandler()
	return nil
}

func (c *Client) idGen() uint16 {
	c.id++
	return uint16(c.id & 0xFFFF)
}

func (c *Client) Subscribe(topics []string, qoss []int) error {
	p := &packets.SubscribePacket{FixedHeader: packets.FixedHeader{MessageType: packets.Subscribe, Qos: 1}}

	p.Topics = topics[:]
	for i := range qoss {
		p.Qoss = append(p.Qoss, byte(qoss[i]))
	}

	p.MessageID = c.idGen()

	ackc := make(chan ACK, 1)
	c.rr[p.MessageID] = ackc

	if c.tracer != nil {
		c.tracer.AddPoint(TraceSubscribe, time.Now())
	}

	if err := p.Write(c.conn); err != nil {
		return err
	}

	ack := <-ackc
	suback := ack.ControlPacket.(*packets.SubackPacket)
	for i, rc := range suback.ReturnCodes {
		if rc == 0x80 {
			return errors.New("Subscribe topic " + topics[i] + " failed")
		}
	}

	return nil
}

func (c *Client) Publish(topic string, message []byte, qos int) (chan ACK, error) {
	p := &packets.PublishPacket{FixedHeader: packets.FixedHeader{MessageType: packets.Publish}}
	p.TopicName = topic
	p.Qos = byte(qos)
	p.Payload = message[:] //copy the slice
	p.MessageID = c.idGen()

	if c.tracer != nil {
		c.tracer.AddPoint(TracePublish, time.Now())
	}

	var ackc chan ACK
	if qos > 0 {
		ackc = make(chan ACK, 1)
		c.rr[p.MessageID] = ackc
	}
	if err := p.Write(c.conn); err != nil {
		return nil, err
	}

	return ackc, nil
}

func (c *Client) recvHandler() {
	for {
		cp, err := packets.ReadPacket(c.conn)
		if err != nil {
			c.errc <- err
		}
		switch p := cp.(type) {
		case *packets.PubackPacket:
			ackc, found := c.rr[p.MessageID]
			if !found {
				continue
			}
			if c.tracer != nil {
				c.tracer.AddPoint(TracePuback, time.Now())
			}
			ackc <- ACK{PacketID: int(p.MessageID), ControlPacket: cp}
		case *packets.SubackPacket:
			ackc, found := c.rr[p.MessageID]
			if !found {
				continue
			}
			if c.tracer != nil {
				c.tracer.AddPoint(TraceSuback, time.Now())
			}
			ackc <- ACK{PacketID: int(p.MessageID), ControlPacket: cp}
		case *packets.PublishPacket:
			if c.tracer != nil {
				c.tracer.AddPoint(TraceMessage, time.Now())
			}
			if f := c.cfg.RecvHandler; f != nil {
				if err := f(p.TopicName, p.Payload, int(p.Qos)); err != nil {
					c.errc <- err
				}
			}
		}
	}
}

func (c *Client) SetRecvHandler(h MessageHandler) {
	c.cfg.RecvHandler = h
}

func (c *Client) Disconnect() error {
	p := &packets.DisconnectPacket{FixedHeader: packets.FixedHeader{MessageType: packets.Disconnect}}
	p.Write(c.conn)
	return c.conn.Close()
}

func (c *Client) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}
func (c *Client) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Client) TracePoints() []*TracePoint {
	return c.tracer.Points()
}
