package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"gitlab.meitu.com/ryq/mqttstat/mqtt"
	"gitlab.meitu.com/ryq/mqttstat/subcmd"
)

const (
	DNSLookupField       = "DNS Lookup"
	TCPConnectionField   = "TCP Connection"
	TLSHandshakeField    = "TLS Handshake"
	MQTTConnectionField  = "MQTT Connection"
	MQTTSubscribeField   = "MQTT Subscribe"
	MQTTPublishField     = "MQTT Publish"
	MQTTMessageRecvField = "MQTT Message Received"
)

const (
	GreenFmt = "\033[32m%v\033[0m"
	GreyFmt  = "\033[0;37m%v\033[0m"
)

type Field struct {
	Name  string
	Cost  string
	Begin string
	End   string
	Len   int
	Time  time.Time
}
type Stat struct {
	fields []*Field
	begin  time.Time
	end    time.Time
}

func parseStat(points []*mqtt.TracePoint) *Stat {
	stat := &Stat{begin: points[0].Time, end: points[len(points)-1].Time}
	ts := make(map[string]time.Time)
	for _, p := range points {
		ts[p.Key] = p.Time
	}

	var last time.Time
	var f *Field

	if t, found := ts[mqtt.TraceDNSLookup]; found {
		field := &Field{Name: DNSLookupField, Begin: "[", End: "", Len: len(DNSLookupField) + 3, Time: t}
		stat.fields = append(stat.fields, field)
		last = t
		f = field
	}

	if t, found := ts[mqtt.TraceTCPDial]; found {
		field := &Field{Name: TCPConnectionField, Begin: "[", End: "", Len: len(TCPConnectionField) + 3, Time: t}
		if len(stat.fields) > 0 {
			field.Begin = "|"
			f.Cost = fmt.Sprint(t.Sub(last))
		}

		stat.fields = append(stat.fields, field)

		last = t
		f = field
	}

	if t, found := ts[mqtt.TraceTLSDial]; found {
		f.Cost = fmt.Sprint(t.Sub(last))

		field := &Field{Name: TLSHandshakeField, Begin: "|", End: "", Len: len(TLSHandshakeField) + 3, Time: t}
		stat.fields = append(stat.fields, field)

		last = t
		f = field
	}

	if t, found := ts[mqtt.TraceConnect]; found {
		f.Cost = fmt.Sprint(t.Sub(last))

		field := &Field{Name: MQTTConnectionField, Begin: "|", End: "]", Len: len(MQTTConnectionField) + 3, Time: t}
		stat.fields = append(stat.fields, field)

		last = t
		f = field
	}

	t := ts[mqtt.TraceConnack]
	f.Cost = fmt.Sprint(t.Sub(last))
	last = t

	if t, found := ts[mqtt.TracePublish]; found {
		f.End = ""

		field := &Field{Name: MQTTPublishField, Begin: "|", End: "]", Len: len(MQTTPublishField) + 3, Time: t}
		stat.fields = append(stat.fields, field)

		last = t
		f = field

		if t, found := ts[mqtt.TracePuback]; found {
			f.Cost = fmt.Sprint(t.Sub(last))
			last = t
		}
	}

	if t, found := ts[mqtt.TraceSubscribe]; found {
		f.End = ""

		field := &Field{Name: MQTTSubscribeField, Begin: "|", End: "]", Len: len(MQTTSubscribeField) + 3, Time: t}
		stat.fields = append(stat.fields, field)

		last = t
		f = field

		if t, found := ts[mqtt.TraceSuback]; found {
			f.Cost = fmt.Sprint(t.Sub(last))
			last = t
		}
	}

	if t, found := ts[mqtt.TraceMessage]; found {
		f.End = ""

		field := &Field{Name: MQTTMessageRecvField, Begin: "|", End: "]", Len: len(MQTTMessageRecvField) + 3, Time: last} // Use last as the start time
		stat.fields = append(stat.fields, field)
		field.Cost = fmt.Sprint(t.Sub(last))

		f = field
		last = t
	}
	return stat
}

func color(colorfmt string, v interface{}) string {
	return fmt.Sprintf(colorfmt, v)
}

func feedSpace(w io.Writer, count int) {
	for i := 0; i < count; i++ {
		w.Write([]byte{' '})
	}
}

func splitSpaceCount(count int) (int, int) {
	if count%2 == 0 {
		c := count / 2
		return c, c
	}
	c := count / 2
	return c, c + 1
}

func (stat *Stat) Display() {
	lines := make([]bytes.Buffer, len(stat.fields)+3) // add extra 3 lines: field, time distribution, and total cost
	start := stat.begin
	total := ""
	offset := 0
	position := 0
	for i, field := range stat.fields {
		//header line
		feedSpace(&lines[0], 2)
		lines[0].WriteString(color(GreyFmt, field.Name))
		feedSpace(&lines[0], 1)

		//cost line
		spaceCount := field.Len - len(field.Begin) - len([]rune(field.Cost))
		if spaceCount < 0 {
			feedSpace(&lines[0], -spaceCount)
			spaceCount = 0
		}

		pre, post := splitSpaceCount(spaceCount)

		lines[1].WriteString(field.Begin)
		feedSpace(&lines[1], pre)
		lines[1].WriteString(color(GreenFmt, field.Cost))
		feedSpace(&lines[1], post)
		lines[1].WriteString(field.End)

		offset += field.Len
		//total cost lines
		for j := 0; j <= i; j++ {
			if j == i {
				feedSpace(&lines[2+j], offset-position-len([]rune(total)))
				lines[2+j].WriteString("|")

				if i == len(stat.fields)-1 {
					total = fmt.Sprint(stat.end.Sub(start))
				} else {
					total = fmt.Sprint(stat.fields[i+1].Time.Sub(start))
				}

				position = offset - len([]rune(total))/2
				feedSpace(&lines[3+j], position)
				lines[3+j].WriteString(color(GreenFmt, total))
			} else {
				feedSpace(&lines[2+j], field.Len-1)
				lines[2+j].WriteString("|")
			}
		}
	}

	for _, line := range lines {
		fmt.Println(line.String())
	}
}

func main() {
	var address string
	cfg := &mqtt.ClientConfig{}
	flag.StringVar(&cfg.Username, "username", "", "username to connect to broker")
	flag.StringVar(&cfg.Password, "password", "", "password of user")
	flag.BoolVar(&cfg.CleanSession, "cleansession", true, "clean session or not")
	flag.StringVar(&cfg.ClientID, "clientid", "mqttstat", "client id of this connection")
	flag.StringVar(&address, "server", "127.0.0.1:1883", "server address")
	flag.Parse()

	cfg.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	c := mqtt.NewClient(cfg)
	if err := c.Dial(address); err != nil {
		log.Fatalln(err)
	}
	args := flag.Args()
	if len(args) > 0 {
		var call subcmd.SubCommand
		switch cmd := args[0]; cmd {
		case "publish":
			call = subcmd.PublishCommand
		case "subscribe":
			call = subcmd.SubscribeCommand
		}

		if err := call(c, args[1:]); err != nil {
			log.Fatalln(err)
		}
	}

	//print the results
	fmt.Println("Connected to", color(GreenFmt, address), "\n")
	if cfg.Username != "" {
		fmt.Println(color(GreyFmt, "Username"), ":", color(GreenFmt, cfg.Username))
	}
	if cfg.Password != "" {
		fmt.Println(color(GreyFmt, "Password"), ":", color(GreenFmt, cfg.Password))
	}
	if cfg.ClientID != "" {
		fmt.Println(color(GreyFmt, "ClientID"), ":", color(GreenFmt, cfg.ClientID))
	}
	fmt.Println(color(GreyFmt, "CleanSession"), ":", color(GreenFmt, cfg.CleanSession))
	fmt.Println()

	parseStat(c.TracePoints()).Display()
}
