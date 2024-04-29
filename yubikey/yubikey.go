// based upon https://github.com/smallstep/crypto
package yubikey

import (
	"crypto"
	"fmt"
	"io"
	"sync"

	"github.com/go-piv/piv-go/piv"
	"github.com/pkg/errors"
	"github.com/twpayne/go-pinentry-minimal/pinentry"
)

type YubiKey struct {
	yk            *piv.YubiKey
	pin           string
	card          string
	managementKey [24]byte
	slot          string
	serial        uint32
}

var (
	pivCards = piv.Cards
	pivMap   sync.Map
)

// pivOpen calls piv.Open. It can be replaced by a custom functions for testing
// purposes.
var pivOpen = func(card string) (*piv.YubiKey, error) {
	return piv.Open(card)
}

// openCard wraps pivOpen with a cache. It loads a card connection from the
// cache if present.
func openCard(card string) (*piv.YubiKey, error) {
	if v, ok := pivMap.Load(card); ok {
		return v.(*piv.YubiKey), nil
	}

	yk, err := pivOpen(card)
	if err != nil {
		return nil, err
	}

	pivMap.Store(card, yk)

	return yk, nil
}

// New initializes a new YubiKey KMS.
func New() (*YubiKey, error) {
	slot := piv.SlotAuthentication.String()

	cards, err := pivCards()
	if err != nil {
		return nil, err
	}

	if len(cards) == 0 {
		return nil, errors.New("error detecting yubikey: try removing and reconnecting the device")
	}

	var yk *piv.YubiKey
	if yk, err = openCard(cards[0]); err != nil {
		return nil, errors.Wrap(err, "error opening yubikey")
	}

	serial, err := yk.Serial()
	if err != nil {
		return nil, errors.Wrap(err, "error getting serial")
	}

	return &YubiKey{
		yk:     yk,
		card:   cards[0],
		slot:   slot,
		serial: serial,
	}, nil
}

func (k *YubiKey) SetSlot(slot string) {
	if slot == "" {
		k.slot = piv.SlotAuthentication.String()
	} else {
		k.slot = slot
	}
}

func (k *YubiKey) GetSlot() string {
	return k.slot
}

// GetPublicKey returns the public key present in the YubiKey signature slot.
func (k *YubiKey) GetPublicKey() (crypto.PublicKey, error) {
	slot, err := getSlot(k.slot)
	if err != nil {
		return nil, err
	}

	pub, err := k.getPublicKey(slot)
	if err != nil {
		return nil, err
	}

	return pub, nil
}

// CreateSigner creates a signer using the key present in the YubiKey signature
// slot.
func (k *YubiKey) CreateSigner() (crypto.Signer, error) {
	slot, err := getSlot(k.slot)
	if err != nil {
		return nil, err
	}

	//	pin := k.pin

	pub, err := k.getPublicKey(slot)
	if err != nil {
		return nil, err
	}

	priv, err := k.yk.PrivateKey(slot, pub, piv.KeyAuth{
		// PIN:       pin,
		// PINPolicy: piv.PINPolicyAlways,
		PINPrompt: k.getPIN,
	})
	if err != nil {
		return nil, errors.Wrap(err, "error retrieving private key")
	}

	signer, ok := priv.(crypto.Signer)
	if !ok {
		return nil, errors.New("private key is not a crypto.Signer")
	}
	return &syncSigner{
		Signer: signer,
	}, nil
}

// Close releases the connection to the YubiKey.
func (k *YubiKey) Close() error {
	if err := k.yk.Close(); err != nil {
		return errors.Wrap(err, "error closing yubikey")
	}
	pivMap.Delete(k.card)
	return nil
}

// means that the key was generated in the device. If not we'll try to get the
// key from a stored certificate in the same slot.
func (k *YubiKey) getPublicKey(slot piv.Slot) (crypto.PublicKey, error) {
	cert, err := k.yk.Attest(slot)
	if err != nil {
		if cert, err = k.yk.Certificate(slot); err != nil {
			return nil, errors.Wrap(err, "error retrieving public key")
		}
	}

	return cert.PublicKey, nil
}

var slotAttestation = piv.Slot{Key: 0xf9, Object: 0x5fff01}

var slotMapping = map[string]piv.Slot{
	"9a": piv.SlotAuthentication,
	"9c": piv.SlotSignature,
	"9e": piv.SlotCardAuthentication,
	"9d": piv.SlotKeyManagement,
	"82": {Key: 0x82, Object: 0x5FC10D},
	"83": {Key: 0x83, Object: 0x5FC10E},
	"84": {Key: 0x84, Object: 0x5FC10F},
	"85": {Key: 0x85, Object: 0x5FC110},
	"86": {Key: 0x86, Object: 0x5FC111},
	"87": {Key: 0x87, Object: 0x5FC112},
	"88": {Key: 0x88, Object: 0x5FC113},
	"89": {Key: 0x89, Object: 0x5FC114},
	"8a": {Key: 0x8a, Object: 0x5FC115},
	"8b": {Key: 0x8b, Object: 0x5FC116},
	"8c": {Key: 0x8c, Object: 0x5FC117},
	"8d": {Key: 0x8d, Object: 0x5FC118},
	"8e": {Key: 0x8e, Object: 0x5FC119},
	"8f": {Key: 0x8f, Object: 0x5FC11A},
	"90": {Key: 0x90, Object: 0x5FC11B},
	"91": {Key: 0x91, Object: 0x5FC11C},
	"92": {Key: 0x92, Object: 0x5FC11D},
	"93": {Key: 0x93, Object: 0x5FC11E},
	"94": {Key: 0x94, Object: 0x5FC11F},
	"95": {Key: 0x95, Object: 0x5FC120},
}

func getSlot(name string) (piv.Slot, error) {
	s, ok := slotMapping[name]
	if !ok {
		return piv.Slot{}, errors.Errorf("unsupported slot-id '%s'", name)
	}

	return s, nil
}

// Common mutex used in syncSigner and syncDecrypter. A sync.Mutex cannot be
// copied after the first use.
//
// By using it, synchronization becomes easier and avoids conflicts between the
// two goroutines accessing the shared resources.
//
// This is not optimal if more than one YubiKey is used, but the overhead should
// be small.
var m sync.Mutex

// syncSigner wraps a crypto.Signer with a mutex to avoid the error "smart card
// error 6982: security status not satisfied" with two concurrent signs.
type syncSigner struct {
	crypto.Signer
}

func (s *syncSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	m.Lock()
	defer m.Unlock()
	return s.Signer.Sign(rand, digest, opts)
}

// syncDecrypter wraps a crypto.Decrypter with a mutex to avoid the error "smart
// card error 6a80: incorrect parameter in command data field" with two
// concurrent decryptions.
type syncDecrypter struct {
	crypto.Decrypter
}

func (k *YubiKey) getPIN() (string, error) {
	serial := k.serial
	retries, _ := k.yk.Retries()

	client, err := pinentry.NewClient(
		pinentry.WithBinaryNameFromGnuPGAgentConf(),
		pinentry.WithGPGTTY(),
		pinentry.WithTitle("ssh-agentx yubikey PIN Prompt"),
		pinentry.WithDesc(fmt.Sprintf("YubiKey serial number: %d (%d tries remaining)", serial, retries)),
		pinentry.WithPrompt("Please enter your PIN:"),
		// Enable opt-in external PIN caching (in the OS keychain).
		// https://gist.github.com/mdeguzis/05d1f284f931223624834788da045c65#file-info-pinentry-L324
		pinentry.WithOption(pinentry.OptionAllowExternalPasswordCache),
		pinentry.WithKeyInfo(fmt.Sprintf("--yubikey-id-%d", serial)),
	)
	if err != nil {
		return "", err
	}
	defer client.Close()

	pin, _, err := client.GetPIN()
	return pin, err
}
