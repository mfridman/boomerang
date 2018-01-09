package auth

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func GetPrivKey(pkFile string) (ssh.Signer, error) {
	// A public key may be used to authenticate against the remote
	// server by using an unencrypted PEM-encoded private key file.
	key, err := ioutil.ReadFile(pkFile)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read private key")
	}

	// Create the Signer for this private key.
	s, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse private key")
	}

	return s, nil
}
