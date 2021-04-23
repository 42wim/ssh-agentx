package main

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/agent"
)

var agentName = "ssh-agentx"

func main() {
	ag := &SSHAgent{
		ExtendedAgent: agent.NewKeyring().(agent.ExtendedAgent),
	}

	v, err := ag.parseConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("# config file not found, continuing as normal ssh-agent")
		} else {
			log.Fatal(err)
		}
	}

	ag.v = v

	ag.start()
}
