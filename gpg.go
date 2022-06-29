package main

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	gorsa "crypto/rsa"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/ProtonMail/go-crypto/openpgp/s2k"
	"github.com/ProtonMail/go-crypto/rsa"
	"golang.org/x/crypto/ssh"
)

type GPGKey struct {
	signer *openpgp.Entity
	pk     ssh.PublicKey
}

func (s *SSHAgent) handleGPGSign(contents []byte) ([]byte, error) {
	var (
		signer *openpgp.Entity
		buf    bytes.Buffer
	)

	uidlen := bytes.IndexByte(contents[:400], 0)
	uid := string(contents[:uidlen])
	data := contents[400:]

	for _, k := range s.gpgkeys {
		if _, ok := k.signer.Identities[uid]; ok {
			signer = k.signer
		}
	}

	if signer == nil {
		log.Printf("no GPG signer found for %s\n", uid)
		return nil, fmt.Errorf("no signer found")
	}

	log.Printf("signing data for %s\n", uid)

	err := openpgp.ArmoredDetachSign(&buf, signer, bytes.NewReader(data), nil)

	return buf.Bytes(), err
}

func (s *SSHAgent) handleGPGRemove(pk ssh.PublicKey) {
	var gpgkeys []GPGKey

	for _, key := range s.gpgkeys {
		if bytes.Equal(key.pk.Marshal(), pk.Marshal()) {
			continue
		}

		gpgkeys = append(gpgkeys, key)
	}

	s.gpgkeys = gpgkeys
}

func (s *SSHAgent) handleGPGImport(privateKey interface{}, comment string) error {
	var gpgkeys []string

	for _, k := range s.v.AllKeys() {
		if !strings.HasPrefix(k, "gpg.") {
			continue
		}

		if !strings.HasSuffix(k, "matchcomment") {
			continue
		}

		gpgkeys = append(gpgkeys, strings.ReplaceAll(k, ".matchcomment", ""))
	}

	for _, key := range gpgkeys {
		if s.v.GetString(key+".matchcomment") != comment {
			continue
		}

		entity, err := s.SSHPrivateKeyToPGP(privateKey, s.v.GetString(key+".name"), s.v.GetString(key+".email"))
		if err != nil {
			return err
		}

		var pk ssh.PublicKey

		switch key := privateKey.(type) {
		case *ed25519.PrivateKey:
			pk, err = ssh.NewPublicKey(ed25519.PublicKey((*key)[32:]))
			if err != nil {
				fmt.Println("pk failed", err)
			}
		case *gorsa.PrivateKey:
			pk, err = ssh.NewPublicKey(key.PublicKey)
			if err != nil {
				fmt.Println("pk failed", err)
			}
		}

		if entity != nil {
			s.gpgkeys = append(s.gpgkeys, GPGKey{
				signer: entity,
				pk:     pk,
			})
		}

		if err != nil {
			fmt.Println("ssh private key failed", err)
		}
	}

	return nil
}

func (s *SSHAgent) SSHPrivateKeyToPGP(privateKey interface{}, name, email string) (*openpgp.Entity, error) {
	config := &packet.Config{
		Algorithm:     packet.PubKeyAlgoEdDSA,
		DefaultHash:   crypto.SHA256,
		DefaultCipher: packet.CipherAES256,
	}

	var primary *packet.PrivateKey

	timeNull := time.Unix(0, 0)

	switch key := privateKey.(type) {
	case *ed25519.PrivateKey:
		config.Algorithm = packet.PubKeyAlgoEdDSA
		primary = packet.NewSignerPrivateKey(timeNull, key)
	case *gorsa.PrivateKey:
		// types are different between protonmail/go-crypto and crypto
		newkey := &rsa.PrivateKey{
			PublicKey: rsa.PublicKey{
				N: key.PublicKey.N,
				E: key.PublicKey.E,
			},
			D:      key.D,
			Primes: key.Primes,
		}
		config.Algorithm = packet.PubKeyAlgoRSA
		primary = packet.NewSignerPrivateKey(timeNull, newkey)
	default:
		return nil, fmt.Errorf("not supported key %T", privateKey)
	}

	keyLifetimeSecs := config.KeyLifetime()

	uid := packet.NewUserId(name, "", email)
	if uid == nil {
		return nil, errors.New("user id field contained invalid characters")
	}

	if config != nil && config.V5Keys {
		primary.UpgradeToV5()
	}

	isPrimaryID := true
	selfSignature := &packet.Signature{
		Version:           primary.PublicKey.Version,
		SigType:           packet.SigTypePositiveCert,
		PubKeyAlgo:        primary.PublicKey.PubKeyAlgo,
		Hash:              config.Hash(),
		CreationTime:      timeNull,
		KeyLifetimeSecs:   &keyLifetimeSecs,
		IssuerKeyId:       &primary.PublicKey.KeyId,
		IssuerFingerprint: primary.PublicKey.Fingerprint,
		IsPrimaryId:       &isPrimaryID,
		FlagsValid:        true,
		FlagSign:          true,
		FlagCertify:       true,
		MDC:               true, // true by default, see 5.8 vs. 5.14
		AEAD:              config.AEAD() != nil,
		V5Keys:            config != nil && config.V5Keys,
	}

	// Set the PreferredHash for the SelfSignature from the packet.Config.
	// If it is not the must-implement algorithm from rfc4880bis, append that.
	selfSignature.PreferredHash = []uint8{hashToHashID(config.Hash())}
	if config.Hash() != crypto.SHA256 {
		selfSignature.PreferredHash = append(selfSignature.PreferredHash, hashToHashID(crypto.SHA256))
	}

	// Likewise for DefaultCipher.
	selfSignature.PreferredSymmetric = []uint8{uint8(config.Cipher())}
	if config.Cipher() != packet.CipherAES128 {
		selfSignature.PreferredSymmetric = append(selfSignature.PreferredSymmetric, uint8(packet.CipherAES128))
	}

	// And for DefaultMode.
	selfSignature.PreferredAEAD = []uint8{uint8(config.AEAD().Mode())}
	if config.AEAD().Mode() != packet.AEADModeEAX {
		selfSignature.PreferredAEAD = append(selfSignature.PreferredAEAD, uint8(packet.AEADModeEAX))
	}

	// User ID binding signature
	err := selfSignature.SignUserId(uid.Id, &primary.PublicKey, primary, config)
	if err != nil {
		return nil, err
	}

	entity := &openpgp.Entity{
		PrimaryKey: &primary.PublicKey,
		PrivateKey: primary,
		Identities: map[string]*openpgp.Identity{
			uid.Id: {
				Name:          uid.Id,
				UserId:        uid,
				SelfSignature: selfSignature,
				Signatures:    []*packet.Signature{selfSignature},
			},
		},
	}

	log.Println("adding public key for", uid.Id)

	writer, err := armor.Encode(os.Stderr, openpgp.PublicKeyType, make(map[string]string))
	if err != nil {
		return nil, fmt.Errorf("failed to encode armor writer: %w", err)
	}

	entity.Serialize(writer)
	writer.Close()
	fmt.Fprintln(os.Stderr, "\n")

	return entity, nil
}

func hashToHashID(h crypto.Hash) uint8 {
	v, ok := s2k.HashToHashId(h)
	if !ok {
		panic("tried to convert unknown hash")
	}

	return v
}
