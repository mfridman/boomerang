package output

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

func init() {
	// Change logging to stdout
	log.SetOutput(os.Stdout)
}

// Finished writes to console upon completion
func Finished(elapsed *time.Duration, machinesIn, machinesOut int) {
	log.Printf("Boomerang completed in %v. input: %v output: %v\n",
		*elapsed-(*elapsed%time.Millisecond),
		machinesIn,
		machinesOut)
}

// CleanUpExcept deletes all .json files in dir except specified files.
// File arguments can be just a name or a absolute path + name.
// Will not panic in the even of an error cleaning up files(s),
// instead the errors are stored in error slice and returned to caller.
func CleanUpExcept(dirName string, files ...string) []error {
	errs := make([]error, 0)

	lookup := make(map[string]bool)
	for _, f := range files {
		_, name := filepath.Split(f)
		lookup[name] = true
	}

	d, err := os.Open(dirName)
	if err != nil {
		errs = append(errs, errors.Wrapf(err, "error opening directory: [%v]", dirName))
		return errs
	}
	defer d.Close()

	fs, err := d.Readdir(0)
	for _, f := range fs {
		// skip files, based on name, that are passed in
		if _, ok := lookup[f.Name()]; ok {
			continue
		}
		// delete files ending in .json in dir
		if f.Mode().IsRegular() && filepath.Ext(f.Name()) == ".json" {
			fn := filepath.Join(dirName, f.Name())
			if err := os.Remove(fn); err != nil {
				errs = append(errs, errors.Wrap(err, "removing JSON file"))
			}
		}
	}

	return errs
}

type OutCfg struct {
	Dir        string
	FilePrefix string
	DateTime   time.Time
}

func (o OutCfg) ToFile() (string, error) {
	filename := o.FilePrefix + "_" + o.DateTime.Format("20060102_150405") + ".json"

	// Check if Dir exists. Create, if necessary, in the current working directory.
	// Make sure Mkdir has permission bit 0744, namely 7. Otherwise os.Create will fail as
	// it cannot enter the dir. Unix tip: the execute bit is necessary to enter a dir
	var checkDir os.FileInfo
	var err error
	if checkDir, err = os.Stat(o.Dir); os.IsNotExist(err) {
		if err := os.Mkdir(o.Dir, 0744); err != nil {
			return "", errors.Wrapf(err, "making directory: [%v]", o.Dir)
		}
		checkDir, err = os.Stat(o.Dir)
		if err != nil {
			return "", errors.Wrapf(err, "getting stat: [%v]", o.Dir)
		}
	}
	if !checkDir.IsDir() {
		return "", errors.Errorf("%v is not a directory. Remove it and let boomerang create its own directory", checkDir.Name())
	}

	return filepath.Join(o.Dir, filename), nil
}
