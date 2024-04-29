package main

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"io/ioutil"
	"log"
	"sync"

	"github.com/42wim/ssh-agentx/yubikey"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var (
	yubiSignExtension      = "ssh-yubi-sign@42wim"
	yubiPublicKeyExtension = "ssh-yubi-publickey@42wim"
	yubiSetSlotExtension   = "ssh-yubi-setslot@42wim"
	gpgSignExtension       = "ssh-gpg-sign@42wim"
)

type SSHAgent struct {
	agent.ExtendedAgent
	gpgkeys    []GPGKey
	v          *viper.Viper
	mutex      sync.RWMutex
	yubisigner crypto.Signer
	yubikey    *yubikey.YubiKey
}

func (s *SSHAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	switch extensionType {
	case gpgSignExtension:
		return s.handleGPGSign(contents)
	case yubiSignExtension:
		if s.v.GetBool("yubikey.enablelog") {
			log.Println("got", extensionType, "request to sign")
		}

		return s.handleYubiSign(contents)
	case yubiPublicKeyExtension:
		if s.v.GetBool("yubikey.enablelog") {
			log.Println("got", extensionType, "request for publickey")
		}

		return x509.MarshalPKIXPublicKey(s.yubisigner.Public())
	case yubiSetSlotExtension:

		var err error

		if s.yubikey.GetSlot() == string(contents) {
			if s.v.GetBool("yubikey.enablelog") {
				if string(contents) == "" {
					log.Println("got", extensionType, "setting slot to default slot (9a) but already set.")
				} else {
					log.Println("got", extensionType, "setting slot to", string(contents), "but already set.")
				}
			}

			return nil, nil
		}

		s.mutex.Lock()
		defer s.mutex.Unlock()

		s.yubikey.SetSlot(string(contents))
		s.yubisigner, err = s.yubikey.CreateSigner()
		if err != nil {
			return nil, err
		}

		if s.v.GetBool("yubikey.enablelog") {
			if string(contents) == "" {
				log.Println("got", extensionType, "setting slot to default slot (9a)")
			} else {
				log.Println("got", extensionType, "setting slot to", string(contents))
			}
			log.Println("got", extensionType, "new crypto signers set")
		}

		return nil, nil
	default:
		return nil, agent.ErrExtensionUnsupported
	}
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

func (s *SSHAgent) handleYubiSign(contents []byte) ([]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.yubisigner.Sign(rand.Reader, contents, crypto.SHA256)
}
