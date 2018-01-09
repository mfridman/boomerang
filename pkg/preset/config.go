package preset

import (
	"log"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Start holds the start time. Intended to track program execution time for output.
var Start = time.Now()

// Cmd structure holds name and command to be executed.
type Cmd struct {
	Name    string
	Command string
}

// CmdSlice holds slice of Cmd. These commands will be executed sequentially on each machine.
var CmdSlice []Cmd

func init() {
	setViperDefaults()
}

// ParseConfigFile reads a file named 'config'. Default is to look for the file in the same directory Boomerang resides.
// The default behaviour can be overriding by passing in the --config file. If a regular, non-empty, file is found it is read in by Viper.
func ParseConfigFile() error {

	viper.SetConfigType("yaml")
	cfgFile := viper.GetString("Config")

	cf, err := os.Stat(cfgFile)
	if err != nil {
		return errors.Wrap(err, "could not get config file stat")
	}

	if !cf.Mode().IsRegular() || cf.Size() == 0 {
		return errors.Errorf("%v is empty [%v] bytes or is not a regular file", cfgFile, cf.Size())
	}

	viper.SetConfigFile(cfgFile)
	if err := viper.ReadInConfig(); err != nil {
		return errors.Wrap(err, "viper could not read in config")
	}

	if err := parseConfigCommands(); err != nil {
		return errors.Wrap(err, "could not parse command slice")
	}

	return nil
}

func parseConfigCommands() error {

	var c []map[interface{}]interface{}

	if err := viper.UnmarshalKey("Commands", &c); err != nil {
		return errors.Wrap(err, "Unable to unmarshal into struct")
	}

	if len(c) == 0 {
		return errors.New("Boomerang has no commands to execute")
	}

	for _, j := range c {
		for k, v := range j {
			if key, ok := k.(string); ok {
				if value, ok := v.(string); ok {
					CmdSlice = append(CmdSlice, Cmd{key, value})
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

	return nil
}
