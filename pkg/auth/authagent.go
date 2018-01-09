package auth

import (
	"net"
	"os"

	"github.com/spf13/viper"

	"github.com/pkg/errors"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// SSHAgent attempts to get unix file via stream socket through environment variable. Default is SSH_AUTH_SOCK,
// but user can override this with agent_ssh_auth option in config.
// If successful will verify key and return an ssh.AuthMethod.
func SSHAgent() (ssh.AuthMethod, error) {

	au := viper.GetString("AgentSSHAuth")

	conn, err := net.Dial("unix", os.Getenv(au))
	if err != nil {
		return nil, errors.Wrapf(err, "dial cannot locate [%v] in env", au)
	}

	// A Signer can create signatures that verify against a public key.
	a, err := agent.NewClient(conn).Signers()
	if err != nil {
		return nil, errors.Wrap(err, "signer failed")
	}

	// If a is empty it is likely this program cannot access the necessary keys,
	// user needs to run ssh-add and authenticate (if key is passphrase-protected)
	if len(a) == 0 {
		return nil, errors.New("unable to authenticate using SSHAgent. Either key not loaded or has passphrase, confirm with ssh-add -l and load with ssh-add")
	}

	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers), nil
}
