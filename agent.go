package main

import (
	"io/ioutil"
	"log"

	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var gpgSignExtension = "ssh-gpg-sign@42wim"

type SSHAgent struct {
	agent.ExtendedAgent
	gpgkeys []GPGKey
	v       *viper.Viper
}

func (s *SSHAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	if extensionType == gpgSignExtension {
		return s.handleGPGSign(contents)
	}

	return nil, agent.ErrExtensionUnsupported
}

func (s *SSHAgent) Add(key agent.AddedKey) error {
	s.handleGPGImport(key.PrivateKey, key.Comment)
	return s.ExtendedAgent.Add(key)
}

func (s *SSHAgent) Remove(key ssh.PublicKey) error {
	s.handleGPGRemove(key)
	return s.ExtendedAgent.Remove(key)
}

func (s *SSHAgent) RemoveAll() error {
	s.gpgkeys = []GPGKey{}
	return s.ExtendedAgent.RemoveAll()
}

func (s *SSHAgent) getSocketDir() string {
	if s.v.GetString("socketdir") != "" {
		return s.v.GetString("socketdir")
	}

	dir, err := ioutil.TempDir("", agentName)
	if err != nil {
		log.Fatal("couldn't create socketdir")
	}

	return dir
}
