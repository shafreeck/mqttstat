package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/mattn/go-isatty"
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
	RedFmt   = "\033[31m%v\033[0m"
	GreyFmt  = "\033[0;37m%v\033[0m"
)

type Field struct {
	Name  string
	Cost  time.Duration
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
			f.Cost = t.Sub(last)
		}

		stat.fields = append(stat.fields, field)

		last = t
		f = field
	}

	if t, found := ts[mqtt.TraceTLSDial]; found {
		f.Cost = t.Sub(last)

		field := &Field{Name: TLSHandshakeField, Begin: "|", End: "", Len: len(TLSHandshakeField) + 3, Time: t}
		stat.fields = append(stat.fields, field)

		last = t
		f = field
	}

	if t, found := ts[mqtt.TraceConnect]; found {
		f.Cost = t.Sub(last)

		field := &Field{Name: MQTTConnectionField, Begin: "|", End: "]", Len: len(MQTTConnectionField) + 3, Time: t}
		stat.fields = append(stat.fields, field)

		last = t
		f = field
	}

	t := ts[mqtt.TraceConnack]
	f.Cost = t.Sub(last)
	last = t

	if t, found := ts[mqtt.TraceSubscribe]; found {
		f.End = ""

		field := &Field{Name: MQTTSubscribeField, Begin: "|", End: "]", Len: len(MQTTSubscribeField) + 3, Time: t}
		stat.fields = append(stat.fields, field)

		last = t
		f = field

		if t, found := ts[mqtt.TraceSuback]; found {
			f.Cost = t.Sub(last)
			last = t
		}
	}

	if t, found := ts[mqtt.TracePublish]; found {
		f.End = ""

		field := &Field{Name: MQTTPublishField, Begin: "|", End: "]", Len: len(MQTTPublishField) + 3, Time: t}
		stat.fields = append(stat.fields, field)

		last = t
		f = field

		if t, found := ts[mqtt.TracePuback]; found {
			f.Cost = t.Sub(last)
			last = t
		}
	}

	if t, found := ts[mqtt.TraceMessage]; found {
		f.End = ""

		field := &Field{Name: MQTTMessageRecvField, Begin: "|", End: "]", Len: len(MQTTMessageRecvField) + 3, Time: last} // Use last as the start time
		stat.fields = append(stat.fields, field)
		field.Cost = t.Sub(last)

		f = field
		last = t
	}
	return stat
}

func color(colorfmt string, v interface{}) string {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		return fmt.Sprintf(colorfmt, v)
	}
	return fmt.Sprint(v)
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

func (stat *Stat) Display(out io.Writer) {
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
		spaceCount := field.Len - len(field.Begin) - len([]rune(fmt.Sprint(field.Cost)))
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
		fmt.Fprintln(out, line.String())
	}
}

//supplied by "li ziang"
func (stat *Stat) ZiangDispaly(out io.Writer) {
	var totalCost, min time.Duration
	min = stat.fields[0].Cost
	for _, field := range stat.fields {
		if min > field.Cost {
			min = field.Cost
		}
		totalCost += field.Cost
	}

	for _, field := range stat.fields {
		fmt.Fprintf(out, "%-21v  %15v\t", field.Name, field.Cost)
		count := int(float64(field.Cost) * 100 / float64(totalCost))
		line := ""
		for i := 0; i < count; i++ {
			line += "█"
		}
		if count > 100/len(stat.fields) {
			fmt.Fprintln(out, color(RedFmt, line))
		} else {
			fmt.Fprintln(out, color(GreenFmt, line))
		}
	}
}

func (stat *Stat) VerticalDisplay() {
	var totalCost, min, sofar time.Duration
	min = stat.fields[0].Cost
	for _, field := range stat.fields {
		if min > field.Cost {
			min = field.Cost
		}
		totalCost += field.Cost
	}

	unit := 1

	fmt.Println(" --")
	for _, field := range stat.fields {
		lineCount := int(field.Cost/min) * unit
		sofar += field.Cost

		for i := 0; i < lineCount; i++ {
			if i == lineCount/2 {
				fmt.Printf(" |    %s (%v)\n", field.Name, color(GreenFmt, field.Cost))
			} else {
				fmt.Println(" | ")
			}
		}
		fmt.Printf(" |<--  %v\n", color(GreenFmt, sofar))
	}
	fmt.Println(" --")
}

func main() {
	var address string
	var sessionTicketEnable, trace, inplace bool
	var count int
	var delay time.Duration

	cfg := &mqtt.ClientConfig{}
	flag.StringVar(&cfg.Username, "username", "", "username to connect to broker")
	flag.StringVar(&cfg.Password, "password", "", "password of user")
	flag.BoolVar(&cfg.CleanSession, "cleansession", true, "clean session or not")
	flag.StringVar(&cfg.ClientID, "clientid", "mqttstat", "client id of this connection")
	flag.StringVar(&address, "server", "127.0.0.1:1883", "server address")
	flag.IntVar(&count, "count", 1, "count to run")
	flag.DurationVar(&delay, "delay", 200*time.Millisecond, "time to delay before next round")
	flag.BoolVar(&trace, "trace", false, "print trace points")
	flag.BoolVar(&inplace, "inplace", false, "keep running and output results inplace")

	flag.IntVar(&cfg.TCPConfig.Linger, "tcp.linger", -1, "set tcp linger")
	flag.IntVar(&cfg.TCPConfig.RecvBuf, "tcp.recvbuf", 0, "tcp recv buffer size")
	flag.IntVar(&cfg.TCPConfig.SendBuf, "tcp.sendbuf", 0, "tcp send buffer size")
	flag.BoolVar(&cfg.TCPConfig.NoDelay, "tcp.nodelay", true, "set tcp nodelay")
	flag.BoolVar(&cfg.TCPConfig.Keepalive, "tcp.keepalive", true, "set tcp keepalive")

	flag.BoolVar(&sessionTicketEnable, "tls.sesstionticket", false, "enable session ticket, works only when connected by tls")
	flag.BoolVar(&cfg.TLSConfig.InsecureSkipVerify, "tls.skipverify", true, "skip server tls verify")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [global options] subcommand [options]\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "global options:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  publish [options]")
		fmt.Fprintln(os.Stderr, "  subscribe [options]")
	}
	flag.Parse()

	if sessionTicketEnable {
		cfg.TLSConfig.ClientSessionCache = tls.NewLRUClientSessionCache(0)
		if count < 2 {
			count = 2
		}
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		ShowCursor()
		os.Exit(0)
	}()

	for i := 0; i < count || inplace; i++ {
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
		c.Disconnect()

		out := bytes.NewBuffer(nil)
		//print the results
		fmt.Fprintln(out, "Connected to", color(GreenFmt, address), "from", c.LocalAddr(), "\n")
		if cfg.Username != "" {
			fmt.Fprintln(out, color(GreyFmt, "Username"), ":", color(GreenFmt, cfg.Username))
		}
		if cfg.Password != "" {
			fmt.Fprintln(out, color(GreyFmt, "Password"), ":", color(GreenFmt, cfg.Password))
		}
		if cfg.ClientID != "" {
			fmt.Fprintln(out, color(GreyFmt, "ClientID"), ":", color(GreenFmt, cfg.ClientID))
		}
		fmt.Fprintln(out, color(GreyFmt, "CleanSession"), ":", color(GreenFmt, cfg.CleanSession))
		fmt.Fprintln(out)

		tracePoints := c.TracePoints()
		if trace {
			OutputTrace(tracePoints)
		}

		stat := parseStat(tracePoints)
		stat.Display(out)
		stat.ZiangDispaly(out)

		if inplace {
			ResetCursor()
		}

		fmt.Print(out)

		if i < count-1 || inplace {
			time.Sleep(time.Duration(delay))
		}
	}
}
func ResetCursor() {
	fmt.Print("\033[1;1H")
	fmt.Print("\033[?25l")
	fmt.Print("\033[2J")
}
func ShowCursor() {
	fmt.Print("\033[?25h")
}

func OutputTrace(points []*mqtt.TracePoint) {
	for _, p := range points {
		fmt.Println(p.Key, p.Time)
	}
}
