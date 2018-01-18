// boomerang executes a list of commands on many machines, concurrently, and return a JSON file
// recording stdout & stderr of each command.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/spf13/pflag"

	"github.com/pkg/errors"
)

// Boomerang is the parent struct written out as JSON to file
type Boomerang struct {
	MetaData    Meta      `json:"metadata"`
	MachineData []Machine `json:"machine_data"`
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

var (
	version = pflag.Bool("version", false, "prints current version")
	config  = pflag.String("c", "config", "specify config file")
)

func main() {

	start := time.Now()

	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	state, err := setup()
	chkErr(err)

	inventory, err := RetrieveInventory(state.inventory)
	chkErr(err)

	/*
		boomerang gets populated throughout the main function and
		passed to output pkg to get written out as a JSON file
	*/
	boomerang := &Boomerang{
		MetaData: Meta{
			BoomerangVersion: VER,
			Type:             state.machineType,
			TotalMachines:    len(inventory),
			Timestamp:        start.Format(time.RFC3339),
		},
		MachineData: make([]Machine, 0),
	}

	var wg sync.WaitGroup
	wg.Add(len(inventory))

	var mut sync.Mutex
	for _, ssh := range inventory {
		go func(s SSHInfo, rc *State) {

			m := NewMachine(s)

			finalMachine := m.Run(rc)

			mut.Lock()
			{
				boomerang.MachineData = append(boomerang.MachineData, *finalMachine)
			}
			mut.Unlock()

			wg.Done()

		}(ssh, state)
	}

	// block until all goroutines have completed.
	wg.Wait()

	/*
		The bulk of the program has completed and all Machine data has been recorded.

		Items below deal with writing Boomerang to a JSON file in the ./raw directory
	*/

	elapsed := time.Since(start)

	boomerang.MetaData.TotalTime = fmt.Sprintf("%v", elapsed-(elapsed%time.Millisecond))

	o := OutCfg{
		Dir:        "raw",
		FilePrefix: state.prefixJSON, // default is raw
		DateTime:   start,
	}

	outFile, err := o.toFile()
	if err != nil {
		log.Fatalln(err)
	}

	f, err := os.Create(outFile)
	if err != nil {
		log.Fatalln(err)
	}
	switch state.indentJSON {
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

	if state.keepLatestFile {
		errs := cleanUpExcept(o.Dir, outFile)
		if len(errs) > 0 {
			for _, e := range errs {
				log.Printf("error cleaning up: %v\n", e)
			}
		}
	}

	finished(&elapsed, len(inventory), len(boomerang.MachineData))
}

func chkErr(e error) {
	if e != nil {
		log.SetPrefix("Boomerang error:\n")
		log.Fatalf("\t%v\n", e)
	}
}
