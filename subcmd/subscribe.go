package subcmd

import (
	"encoding/base64"
	"errors"
	"flag"
	"log"
	"strconv"
	"strings"

	"github.com/shafreeck/mqttstat/mqtt"
)

func SubscribeCommand(c *mqtt.Client, args []string) error {
	var topic, pub string
	var qos string
	var wait, verbose bool

	fs := flag.NewFlagSet("subscribe", flag.ExitOnError)
	fs.StringVar(&topic, "topic", "/mqttstat", "topic to publish message to")
	fs.StringVar(&pub, "pub", "", "publish message to the first topic, the content should be encoded by base64")
	fs.StringVar(&qos, "qos", "1", "qos of message")
	fs.BoolVar(&wait, "wait", false, "wait for the first message")
	fs.BoolVar(&verbose, "v", false, "verbose")
	if err := fs.Parse(args); err != nil {
		return err
	}

	topics := strings.Split(topic, ",")
	rawqoss := strings.Split(qos, ",")
	if len(topics) != len(rawqoss) {
		return errors.New("size of topics and qoss does not match")
	}

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
			if verbose {
				log.Printf("topic: %s, message size: %d, qos: %d", topic, len(message), qos)
			}
			waitc <- struct{}{}
			return nil
		})
	}

	if err := c.Subscribe(topics, qoss); err != nil {
		return err
	}

	//publish messge before subscription, sometimes we want to upload some init message first
	if pub != "" {
		msg, err := base64.StdEncoding.DecodeString(pub)
		if err != nil {
			log.Fatalln(err)
		}

		ackc, err := c.Publish(topics[0], msg, 1)
		if err != nil {
			return err
		}
		<-ackc
	}

	if wait {
		<-waitc
	}
	return nil
}
