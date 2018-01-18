package main

import (
	"bufio"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// setAuth accepts auth options and attempts converts auth to an ssh.AuthMethod.
// Supports key, agent or password.
// If using auth=key must supply privKeyLocation,
// If using auth=password must supply password.
// if using auth=agent, default is SSH_AUTH_SOCK.
func setAuth(a authOpt) (ssh.AuthMethod, error) {

	switch a.auth {
	case "key":
		// check existence of private key in config file
		pk := a.key
		if pk == "" {
			return nil, errors.New("must include privKeyLocation when auth=key")
		}

		signer, err := getPrivKey(pk) // get private key
		if err != nil {
			return nil, errors.Wrapf(err, "could not convert private key to a valid signer: %s", pk)
		}

		return ssh.PublicKeys(signer), nil

	case "agent":
		auth, err := sshAgent(a.agent)
		if err != nil {
			return nil, errors.Wrapf(err, "could not convert agent into a valid auth method: %s", a.agent)
		}
		return auth, nil

	case "password":
		// check existence of private key in config file
		p := a.pass
		if p == "" {
			return nil, errors.New("must include SSHpassword when auth=password")
		}

		return ssh.Password(p), nil

	default:
		return nil, errors.Errorf("unsupported auth method: %v\n\tmust use key, agent or password", a.auth)
	}

}

func sshAgent(s string) (ssh.AuthMethod, error) {

	conn, err := net.Dial("unix", os.Getenv(s))
	if err != nil {
		return nil, errors.Wrapf(err, "could not get %s from env", s)
	}

	// A Signer can create signatures that verify against a public key.
	ss, err := agent.NewClient(conn).Signers()
	if err != nil {
		return nil, errors.Wrap(err, "signer failed")
	}

	// If ss is empty program cannot access the necessary keys,
	// user may need to run ssh-add and authenticate (if key is passphrase-protected)
	if len(ss) == 0 {
		return nil, errors.Errorf("unable to authenticate agent using [%v]. Either key not loaded or has passphrase, confirm with ssh-add -l and load with ssh-add", s)
	}

	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers), nil
}

func getPrivKey(pkFile string) (ssh.Signer, error) {
	// A public key may be used to authenticate against the remote
	// server by using an unencrypted PEM-encoded private key file.
	b, err := ioutil.ReadFile(pkFile)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read private key")
	}

	// Create the Signer for this private key.
	s, err := ssh.ParsePrivateKey(b)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse private key to a Signer")
	}

	return s, nil
}

func checkHostKey(host, port string) (ssh.PublicKey, error) {
	file, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var hostport string
	if port == "22" {
		hostport = host
	} else {
		hostport = "[" + host + "]:" + port
	}

	scanner := bufio.NewScanner(file)
	var hostKey ssh.PublicKey
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), " ")
		if len(fields) != 3 {
			continue
		}
		if strings.Contains(fields[0], hostport) {
			var err error
			hostKey, _, _, _, err = ssh.ParseAuthorizedKey(scanner.Bytes())
			if err != nil {
				return nil, errors.Wrapf(err, "error parsing [%q]", fields[2])
			}
			break
		}
	}

	if hostKey == nil {
		cause := errors.New("no hostkey")
		return nil, errors.Wrapf(cause, "[%v]", host+":"+port)
	}
	return hostKey, nil

}
