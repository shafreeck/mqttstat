package subcmd

import (
	"flag"
	"fmt"
	"gitlab.meitu.com/ryq/mqttstat/mqtt"
)

func PublishCommand(c *mqtt.Client, args []string) error {
	var topic, message string
	var qos int
	var verbose bool

	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	fs.StringVar(&topic, "topic", "/mqttstat", "topic to publish message to")
	fs.StringVar(&message, "message", "mqttstat test", "content of message")
	fs.IntVar(&qos, "qos", 1, "qos of message")
	fs.BoolVar(&verbose, "v", false, "verbose")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ackc, err := c.Publish(topic, []byte(message), qos)
	if err != nil {
		return err
	}

	ack := <-ackc
	if verbose {
		fmt.Println(ack.ControlPacket)
		fmt.Println()
	}
	return nil
}
