// boomerang executes a list of commands on many machines, concurrently, and return a JSON file
// recording stdout & stderr of each command.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"

	"github.com/pkg/errors"

	"github.com/mfridman/boomerang/pkg/machine"
	"github.com/mfridman/boomerang/pkg/output"
	pre "github.com/mfridman/boomerang/pkg/preset"
)

// VERSION holds the current version+build, which is defined at build time.
var VERSION = "undefined"

// Boomerang is the parent struct written out as JSON to file
type Boomerang struct {
	MetaData    Meta              `json:"metadata"`
	MachineData []machine.Machine `json:"machine_data"`
}

func (b *Boomerang) writeJSON(w io.Writer) error {
	by, err := json.Marshal(b)
	if err != nil {
		return errors.Wrap(err, "failed marshal")
	}
	if _, err := w.Write(by); err != nil {
		return err
	}
	return nil
}

func (b *Boomerang) writeIndentJSON(w io.Writer) error {
	by, err := json.MarshalIndent(b, "", "\t")
	if err != nil {
		return errors.Wrap(err, "failed marshal")
	}
	if _, err := w.Write(by); err != nil {
		return err
	}
	return nil
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
			Timestamp:        viper.GetTime("ProgStartTime").Format(time.RFC3339),
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

	var mut sync.Mutex
	for _, ssh := range inventory {
		go func(s machine.SSHInfo, rc machine.RunConfig) {

			m := machine.NewMachine(s)

			finalMachine := m.Run(rc)

			mut.Lock()
			{
				boomerang.MachineData = append(boomerang.MachineData, *finalMachine)
			}
			mut.Unlock()

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

	o := output.OutCfg{
		Dir:        "raw",
		FilePrefix: viper.GetString("PrefixJSON"), // default is raw
		DateTime:   viper.GetTime("ProgStartTime"),
	}

	outFile, err := o.ToFile()
	if err != nil {
		log.Fatalln(err)
	}

	f, err := os.Create(outFile)
	if err != nil {
		log.Fatalln(err)
	}
	switch viper.GetBool("IndentJSON") {
	case true:
		if err := boomerang.writeIndentJSON(f); err != nil {
			log.Fatalln(err)
		}
	case false:
		if err := boomerang.writeJSON(f); err != nil {
			log.Fatalln(err)
		}
	}
	if err := f.Close(); err != nil {
		log.Fatalln(err)
	}

	if viper.GetBool("KeepLatestFileOnly") {
		errs := output.CleanUpExcept(o.Dir, outFile)
		if len(errs) > 0 {
			for _, e := range errs {
				log.Printf("error cleaning up: %v\n", e)
			}
		}
	}

	output.Finished(&elapsed, len(inventory), len(boomerang.MachineData))
}
