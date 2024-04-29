package main

import (
	"fmt"
	"log"

	"github.com/42wim/ssh-agentx/yubikey"
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

	if v.GetBool("yubikey.enable") {
		yubi, err := yubikey.New()
		if err != nil {
			panic(err)
		}

		if v.GetBool("yubikey.enablelog") {
			log.Println("setting slot to", v.GetString("yubikey.defaultslot"))
		}

		yubi.SetSlot(v.GetString("yubikey.defaultslot"))

		y, err := yubi.CreateSigner()
		if err != nil {
			panic(err)
		}

		ag.yubikey = yubi
		ag.yubisigner = y
	}

	ag.v = v

	ag.start()
}
