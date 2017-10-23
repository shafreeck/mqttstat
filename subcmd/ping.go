package subcmd

import (
	"flag"
	"fmt"

	"gitlab.meitu.com/ryq/mqttstat/mqtt"
)

func PingCommand(c *mqtt.Client, args []string) error {
	var verbose bool

	fs := flag.NewFlagSet("ping", flag.ExitOnError)
	fs.BoolVar(&verbose, "v", false, "verbose")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ch, err := c.Ping()
	if err != nil {
		return err
	}

	pong := <-ch
	if verbose {
		fmt.Println(pong)
		fmt.Println()
	}
	return nil
}
