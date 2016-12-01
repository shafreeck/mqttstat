package subcmd

import (
	"flag"
	"strconv"
	"strings"

	"gitlab.meitu.com/ryq/mqttstat/mqtt"
)

func SubscribeCommand(c *mqtt.Client, args []string) error {
	var topic string
	var qos string
	var wait bool

	fs := flag.NewFlagSet("subscribe", flag.ExitOnError)
	fs.StringVar(&topic, "topic", "/mqttstat", "topic to publish message to")
	fs.StringVar(&qos, "qos", "1", "qos of message")
	fs.BoolVar(&wait, "wait", false, "wait for the first message")
	if err := fs.Parse(args); err != nil {
		return err
	}

	topics := strings.Split(topic, ",")
	rawqoss := strings.Split(qos, ",")

	qoss := make([]int, len(rawqoss))
	for i, v := range rawqoss {
		qos, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		qoss[i] = qos
	}

	waitc := make(chan struct{}, 1)
	//wait for the first message
	if wait {
		c.SetRecvHandler(func(topic string, message []byte, qos int) error {
			waitc <- struct{}{}
			return nil
		})
	}

	if err := c.Subscribe(topics, qoss); err != nil {
		return err
	}

	if wait {
		<-waitc
	}
	return nil
}
