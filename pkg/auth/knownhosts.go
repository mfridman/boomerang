package auth

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func CheckHostKey(host, port string) (ssh.PublicKey, error) {
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
