package subcmd

import (
	"github.com/shafreeck/mqttstat/mqtt"
)

type SubCommand func(c *mqtt.Client, args []string) error
