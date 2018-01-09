package output

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// TimeStamp is the global timestamp
var TimeStamp = time.Now()

func init() {
	// Change logging to stdout
	log.SetOutput(os.Stdout)
}

// WriteJSON writes out parent Boomerang struct to a file
// TODO, consider passing in jConf and decouple viper options to make it more generic
// i.e., support any interface and any set of options.
func WriteJSON(data interface{}) error {
	var j []byte
	var err error

	switch viper.GetBool("IndentJSON") {
	case true:
		j, err = json.MarshalIndent(data, "", "\t")
		if err != nil {
			return errors.Wrap(err, "Failed marshaling with indent")
		}
	case false:
		j, err = json.Marshal(data)
		if err != nil {
			return errors.Wrap(err, "Failed marshaling")
		}
	}

	// get absolute working path. This is where raw dir will be created
	wd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "Error getting current directory")
	}

	wdRawDir := filepath.Join(wd, "raw")
	filename := viper.GetString("PrefixJSON") + "_" + TimeStamp.Format("20060102_150405") + ".json"

	// check raw dir existence, create if necessary in the current working directory
	// make sure Mkdir has permission bit 0744, namely 7. Otherwise os.Create will fail as
	// it cannot enter the dir. Unix 101: the execute bit is necessary to enter a dir
	var checkRaw os.FileInfo
	if checkRaw, err = os.Stat(wdRawDir); os.IsNotExist(err) {
		if err := os.Mkdir(wdRawDir, 0744); err != nil {
			return errors.Wrap(err, "Error making raw data directory")
		}
		checkRaw, err = os.Stat(wdRawDir)
		if err != nil {
			return errors.Wrapf(err, "Error getting raw data dir stat [%v]", wdRawDir)
		}
	}

	if !checkRaw.IsDir() {
		return errors.Errorf("Error: %v is not a directory. Remove it and let boomerang create its own folder", checkRaw.Name())
	}

	fullPath := filepath.Join(wdRawDir, filename)

	switch viper.GetBool("KeepLatestFileOnly") {
	case true:
		errs := cleanupJSON(wdRawDir)
		// cleanupJSON will not panic in the event there is an error cleaning up files(s).
		// Instead the errors are logged to Stdout.
		if len(errs) > 0 {
			for _, e := range errs {
				log.Printf("Error cleaning up: %v\n", e)
			}
		}

		if err := mkJSON(fullPath, j); err != nil {
			return errors.Wrap(err, "mkJSON failed")
		}
	case false:
		if err := mkJSON(fullPath, j); err != nil {
			return errors.Wrap(err, "mkJSON failed")
		}
	}

	return nil
}

// Finished writes to console upon completion
func Finished(elapsed *time.Duration, machinesIn, machinesOut int) {
	log.Printf("Boomerang completed in %v. input: %v output: %v\n",
		*elapsed-(*elapsed%time.Millisecond),
		machinesIn,
		machinesOut)
}

func cleanupJSON(dir string) []error {
	var errs []error

	d, err := os.Open(dir)
	if err != nil {
		errs = append(errs, errors.Wrapf(err, "Error opening [%v]", dir))
		return errs
	}
	defer d.Close()

	files, err := d.Readdir(0)
	if err != nil {
		errs = append(errs, errors.Wrapf(err, "Error reading files from [%v]", dir))
		return errs
	}

	for _, j := range files {
		if j.Mode().IsRegular() && filepath.Ext(j.Name()) == ".json" {
			fn := filepath.Join(dir, j.Name())
			if err := os.Remove(fn); err != nil {
				errs = append(errs, errors.Wrap(err, "Error removing old JSON file"))
			}
		}
	}

	return errs
}

func mkJSON(file string, j []byte) error {
	fo, err := os.Create(file) // fo is *File with an absolute path and name

	if err != nil {
		return fmt.Errorf("Error creating file: %v", err)
	}

	if _, err := fo.Write(j); err != nil {
		return fmt.Errorf("Error writing to file: %v", err)
	}

	if err := fo.Close(); err != nil {
		return fmt.Errorf("Error closing file: %v", err)
	}

	return nil
}
