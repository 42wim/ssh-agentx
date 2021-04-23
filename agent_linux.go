package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh/agent"
)

func (s *SSHAgent) start() {
	socketDir := s.getSocketDir()
	socketFile := filepath.Join(socketDir, "agent.sock")

	defer func() {
		os.Remove(socketFile)
		os.Remove(socketDir)
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		for range c {
			os.Remove(socketFile)
			os.Remove(socketDir)
			os.Exit(0)
		}
	}()

	l, err := net.Listen("unix", socketFile)
	if err != nil {
		log.Fatalln("Failed to listen on UNIX socket:", err)
	}

	fmt.Println("SSH_AUTH_SOCK=" + socketFile + "; export SSH_AUTH_SOCK;")
	fmt.Println("SSH_AGENT_PID=" + strconv.Itoa(os.Getpid()) + "; export SSH_AGENT_PID;")
	fmt.Println("echo Agent pid " + strconv.Itoa(os.Getpid()) + ";")

	for {
		c, err := l.Accept()
		if err != nil {
			type temporary interface {
				Temporary() bool
			}

			if err, ok := err.(temporary); ok && err.Temporary() {
				log.Println("Temporary Accept error, sleeping 1s:", err)
				time.Sleep(1 * time.Second)

				continue
			}

			log.Fatalln("Failed to accept connections:", err)
		}

		go func() {
			if err := agent.ServeAgent(s, c); err != io.EOF {
				log.Println("Agent client connection ended with error:", err)
			}
		}()
	}
}
