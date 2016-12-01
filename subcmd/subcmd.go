package subcmd

import (
	"gitlab.meitu.com/ryq/mqttstat/mqtt"
)

type SubCommand func(c *mqtt.Client, args []string) error
