package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
)

var (
	// VER holds version+build, which is defined at build time through -ldflags.
	// Example: -ldflags="-X main.VERSION=$(git describe --always --abbrev=0 --tags)+$(git rev-parse --short=8 master)"
	VER = "devel"
)

func setup() (*State, error) {
	// set program defaults
	setViperDefaults()

	// adds *flag.FlagSet to the pflag.FlagSet
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	// bind full flag set to the configuration.
	// Uses each flag's long name as the config key
	viper.BindPFlags(pflag.CommandLine)

	if len(pflag.Args()) > 0 {
		return nil, errors.New("boomerang does not accept args. Define state using flags or config file")
	}

	if *version {
		fmt.Fprintf(os.Stdout, "boomerang version+hash: %s\n", VER)
		os.Exit(0)
	}

	// Before initializing State and converting all viper options to State, read in a config file.
	// Most options can be specified through cli flags, but, config is a mandatory requirement
	// because it contains a list of commands to execute.
	if err := readConfig(*config); err != nil {
		return nil, err
	}

	state := newState()

	if err := state.importFromViper(); err != nil {
		return nil, err
	}

	return state, nil
}

// setViperDefaults sets sensible defaults.
func setViperDefaults() {
	viper.SetDefault("hostKeyCheck", true)
	viper.SetDefault("machineType", "")
	viper.SetDefault("keepLatestFile", false)
	viper.SetDefault("indentJSON", true)
	viper.SetDefault("prefixJSON", "raw")
	viper.SetDefault("connTimeout", 10)
	viper.SetDefault("retry", 1)
	viper.SetDefault("retryWait", 15)
	viper.SetDefault("agentSSHAuth", "SSH_AUTH_SOCK")
}

// State holds all necessary information for Boomerang to run.
// Once setup no fields are mutable.
type State struct {
	configFile       string         // mandatory
	inventory        string         // mandatory
	auth             ssh.AuthMethod // mandatory
	privKeyLocation  string         // conditional
	SSHpassword      string         // conditional
	agentSSHAuth     string
	machineType      string
	prefixJSON       string
	connTimeout      int64 // TODO, convert this to duration
	retry, retryWait int64 // TODO, convert this to duration
	hostKeyCheck     bool
	keepLatestFile   bool
	indentJSON       bool
	commands         []command
}

type command struct {
	name string
	cmd  string
	sudo bool
}

// newState returns State.
func newState() *State {
	s := &State{
		commands: make([]command, 0),
	}
	return s
}

// fromViper updates State based on viper options
func (s *State) importFromViper() error {

	// config
	s.configFile = viper.GetString("config") // from cli flag or from current dir

	// inventory
	if !viper.IsSet("inventory") {
		return errors.New("missing inventory option")
	}
	s.inventory = viper.GetString("inventory")

	// authentication method
	if !viper.IsSet("auth") {
		return errors.New("missing valid auth option. Available options: key, agent or password")
	}

	opts := authOpt{
		auth:  viper.GetString("auth"),
		key:   viper.GetString("privKeyLocation"),
		pass:  viper.GetString("SSHpassword"),
		agent: viper.GetString("agentSSHAuth"),
	}

	a, err := setAuth(opts)
	if err != nil {
		return err
	}
	s.auth = a

	s.machineType = viper.GetString("machineType")
	s.prefixJSON = viper.GetString("prefixJSON")

	if viper.GetInt64("connTimeout") < 0 || viper.GetInt64("retry") < 0 || viper.GetInt64("retryWait") < 0 {
		return errors.New("connTimeout, retryWait or retry must be a positive value")
	}
	s.connTimeout = viper.GetInt64("connTimeout")
	s.retry = viper.GetInt64("retry")
	s.retryWait = viper.GetInt64("retryWait")

	s.hostKeyCheck = viper.GetBool("hostKeyCheck")
	s.keepLatestFile = viper.GetBool("keepLatestFile")
	s.indentJSON = viper.GetBool("indentJSON")

	c := viper.Get("commands")

	v, ok := c.([]command)
	if !ok {
		return errors.New("could not assert command list")
	}
	s.commands = v

	return nil
}

// readConfig reads config file and stores commands and suser options in viper.
func readConfig(f string) error {

	viper.SetConfigType("yaml")

	cfg, err := os.Stat(f)
	if err != nil {
		return errors.Errorf("could not locate config file: %s\n", f)
	}

	if !cfg.Mode().IsRegular() || cfg.Size() == 0 {
		return errors.Errorf("%s contains [%d] bytes and may be empty or is not a regular file", f, cfg.Size())
	}

	viper.SetConfigFile(f)
	if err := viper.ReadInConfig(); err != nil {
		return errors.Wrap(err, "viper could not read in config")
	}

	if err := parseCommands(); err != nil {
		return err
	}

	return nil
}

func parseCommands() error {

	i := make([]map[interface{}]interface{}, 0)

	out := make([]command, 0)

	if !viper.InConfig("commands") {
		return errors.New("could not find commands key in config file")
	}

	if err := viper.UnmarshalKey("commands", &i); err != nil {
		return errors.Wrap(err, "unable to unmarshal commands into struct")
	}

	if len(i) == 0 {
		return errors.New("no commands specified in config file")
	}

	for _, m := range i {
		for k, v := range m {
			if key, ok := k.(string); ok {
				if value, ok := v.(string); ok {
					b := strings.Contains(value, "sudo")
					out = append(out, command{key, value, b})
				} else {
					log.Printf("Warning: [%v] is not a string. Command will be ignored, check config file\n", v)
					continue
				}
			} else {
				log.Printf("Warning: [%v] is not a string. Command name will be ignored, check config file\n", k)
				continue
			}
		}
	}

	if len(out) == 0 {
		return errors.New("could not generate list of commands from config file")
	}

	viper.Set("commands", out)

	return nil
}

type authOpt struct {
	auth  string
	key   string
	pass  string
	agent string
}
