package preset

import (
	"io/ioutil"

	"golang.org/x/crypto/ssh"

	"github.com/mfridman/boomerang/pkg/auth"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// ValidateViperOptions validates some config options. It also calls functions that read inventory from either
// a network or a local file to setup a slice of Machines. The auth method is also established here.
// Errors are returned up the call stack to main.
//
func ValidateViperOptions() error {

	if err := valAuth(); err != nil {
		return err
	}

	if err := valConnTimeout(); err != nil {
		return err
	}

	return nil
}

func valConnTimeout() error {
	if viper.GetInt64("ConnTimeout") < 0 {
		viper.Set("ConnTimeout", 30)
	}

	return nil
}

// valAuth checks config for a user-defined auth and sets internal viper option userAuthMethod to an ssh.AuthMethod
// supports key, agent or password. If using auth=key must supply key_location,
// if using auth=password must supply password.
func valAuth() error {

	if !viper.IsSet("Auth") {
		return errors.New("no valid auth method found, add auth option to config file")
	}

	switch viper.GetString("Auth") {
	case "key":
		// check existence of private key in config file
		if !viper.IsSet("PrivKey") {
			return errors.New("must include key_location when auth=key")
		}

		pk := viper.GetString("PrivKey")

		if _, err := ioutil.ReadFile(pk); err != nil {
			return errors.Wrapf(err, "unable read private key: [%v]", pk)
		}

		signer, err := auth.GetPrivKey(pk) // get private key
		if err != nil {
			return errors.Wrapf(err, "could not get private key: [%v]", pk)
		}

		viper.Set("userAuthMethod", ssh.PublicKeys(signer))
		return nil

	case "agent":
		a, err := auth.SSHAgent()
		if err != nil {
			return errors.Wrap(err, "could not setup a userAuthMethod")
		}

		viper.Set("userAuthMethod", a)
		return nil

	case "password":
		// check existence of password in config file
		if !viper.IsSet("Password") {
			return errors.New("must include password when auth=password")
		}

		pass := viper.GetString("Password")
		if pass == "" {
			return errors.New("when using auth=password, must supply valid password in config file")
		}

		viper.Set("userAuthMethod", ssh.Password(pass))
		return nil

	default:
		return errors.Errorf("auth=%v unsupported. Must use key, agent or password", viper.GetString("Auth"))
	}

}
