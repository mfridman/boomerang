// boomerang executes a list of commands on many machines, concurrently, and return a JSON file
// recording stdout & stderr of each command.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/pkg/errors"

	"github.com/mfridman/boomerang/pkg/machine"
	"github.com/mfridman/boomerang/pkg/output"
	pre "github.com/mfridman/boomerang/pkg/preset"
)

// VERSION holds the current version+build, which is defined at build time.
var VERSION = "undefined"

// Boomerang is the parent struct written out as JSON to file
type Boomerang struct {
	MetaData Meta `json:"metadata"`

	machineMu   sync.Mutex
	MachineData []machine.Machine `json:"machine_data"`
}

// Meta structure holds all non machine-specific data
type Meta struct {
	// TODO remove BoomerangVersion once API becomes stable,
	// used mainly for debugging as API change frequently.
	// Think about replacing with an actual API version?
	BoomerangVersion string `json:"boomerang_version"`
	Type             string `json:"type"`
	Timestamp        string `json:"timestamp"`
	TotalMachines    int    `json:"total_items"`
	TotalTime        string `json:"total_time"`
}

func main() {

	ver := pflag.Bool("version", false, "prints current version")

	// set default config file name, otherwise user must specify via command line
	pflag.String("config", "config", "override default config file")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	if *ver {
		fmt.Println("Boomerang version", VERSION)
		os.Exit(0)
	}

	if err := pre.ParseConfigFile(); err != nil {
		log.Fatalln(errors.Wrap(err, "Fatal: Error config file setup"))
	}

	if !viper.IsSet("inventory") {
		log.Fatalln("Error: config file is missing inventory option")
	}

	if err := pre.ValidateViperOptions(); err != nil {
		log.Fatalln(errors.Wrap(err, "Fatal: Error validating options"))
	}

	authMethod, ok := viper.Get("userAuthMethod").(ssh.AuthMethod)
	if !ok {
		log.Fatalf("Fatal: Failed ssh.AuthMethod type assertion, got [%T] exepcting [ssh.AuthMethod]. Check valAuth()\n", viper.Get("userAuthMethod"))
	}

	inventory, err := machine.RetrieveInventory(viper.GetString("inventory"))
	if err != nil {
		log.Fatalln(err)
	}

	/*
		boomerang gets populated throughout the main function and
		passed to output pkg to get written out as a JSON file
	*/
	boomerang := &Boomerang{
		MetaData: Meta{
			BoomerangVersion: VERSION,
			Type:             viper.GetString("Type"),
			Timestamp:        output.TimeStamp.Format(time.RFC3339),
			TotalMachines:    len(inventory),
		},
		MachineData: make([]machine.Machine, 0),
	}

	runCfg := machine.RunConfig{
		HostKeyCheck:   viper.GetBool("HostKeyCheck"),
		ConnTimeout:    viper.GetInt64("ConnTimeout"),
		UserAuthMethod: authMethod,
		Retry:          viper.GetInt64("Retry"),
		RetryWait:      viper.GetInt64("RetryWait"),
	}

	var wg sync.WaitGroup
	wg.Add(len(inventory))

	for _, ssh := range inventory {
		go func(s machine.SSHInfo, rc machine.RunConfig) {

			m := machine.NewMachine(s)

			finalMachine := m.Run(rc)

			boomerang.machineMu.Lock()
			{
				boomerang.MachineData = append(boomerang.MachineData, *finalMachine)
			}
			boomerang.machineMu.Unlock()

			wg.Done()

		}(ssh, runCfg)
	}

	// block until all goroutines have completed.
	wg.Wait()

	/*
		The bulk of the program has completed and all Machine data has been recorded.

		Items below deal with writing Boomerang to a JSON file in the ./raw directory
	*/

	elapsed := time.Since(pre.Start)

	boomerang.MetaData.TotalTime = fmt.Sprintf("%v", elapsed-(elapsed%time.Millisecond))
	if err := output.WriteJSON(boomerang); err != nil {
		log.Fatalln(err)
	}

	output.Finished(&elapsed, len(inventory), len(boomerang.MachineData))
}
