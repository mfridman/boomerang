package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/pkg/errors"
)

// SSHInfo stores machine-specific information required for establishing an SSH connection.
// Username & hostname are mandatory. If left unspecified, port will default to 22.
// Extras are optional and will be written out as-is.
type SSHInfo struct {
	HostName string                 `json:"hostname"`
	Username string                 `json:"username"`
	Port     string                 `json:"ssh_port"`
	Extras   map[string]interface{} `json:"extras"`
}

// The Machine struct contains all information related to a specific machine.
//
// This includes the initial machine ssh information required for establsihing a connection and
// all subsequent data related to command(s) execution.
type Machine struct {
	Connection       bool     `json:"connection"`
	RunLength        float64  `json:"run_length"`
	ConnectionErrors []string `json:"connection_errors"`
	StreamData       []Stream `json:"stream_data"`
	SSHInfo
}

// Stream captures data from each ssh session run
type Stream struct {
	Name         string   `json:"name"`
	Stdout       string   `json:"stdout"`
	Stderr       string   `json:"stderr"`
	ExitCode     int      `json:"exit_code"`
	StreamErrors []string `json:"stream_errors"`
}

// NewMachine returns a pointer to an initialized Machine struct.
func NewMachine(s SSHInfo) *Machine {
	m := Machine{
		ConnectionErrors: make([]string, 0),
		StreamData:       make([]Stream, 0),
		SSHInfo:          s,
	}
	if m.Extras == nil {
		m.Extras = make(map[string]interface{}, 0)
	}
	return &m
}

// RetrieveInventory retrieves an inventory of machine ssh info based on the location string.
// The location string must be a local file or a network address.
//
// If supplying a network address, it must have the prefix http or https.
// The default timeout for the underlying Get request is 10s.
//
// If supplying a filename, it must be located in the same directory as Boomerang.
// Otherwise must supply the full path to the file. Avoid file names with the prefix
// http or https.
func RetrieveInventory(l string) ([]SSHInfo, error) {

	re, err := regexp.Compile(`^(http|https)://`)
	if err != nil {
		return nil, errors.Wrap(err, "error compiling regex")
	}

	if re.MatchString(l) {
		ssh, err := getInventoryFromURL(l)
		if err != nil {
			return nil, errors.Wrap(err, "could not get inventory from url")
		}
		return ssh, nil
	}

	ssh, err := getInventoryFromFile(l)
	if err != nil {
		return nil, errors.Wrap(err, "could not get inventory from file")
	}

	return ssh, nil
}

func getInventoryFromURL(url string) ([]SSHInfo, error) {

	var inventory []SSHInfo

	c := &http.Client{Timeout: time.Duration(10 * time.Second)}
	resp, err := c.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "unable to fetch url")
	}

	if resp.StatusCode != 200 {
		return nil, errors.Errorf("server returned a [%v], expecting status code 200", resp.Status)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&inventory); err != nil {
		return nil, errors.Wrapf(err, "could not decode inventory from [%v]", url)
	}

	return inventory, nil
}

func getInventoryFromFile(file string) ([]SSHInfo, error) {

	var inventory []SSHInfo

	if !fileExists(file) {
		return nil, errors.Errorf("stat on file failed or file does not exist: check %v", file)
	}

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&inventory); err != nil {
		return nil, errors.Wrapf(err, "could not decode inventory from [%v]", file)
	}

	return inventory, nil
}

func fileExists(f string) bool {
	_, err := os.Stat(f)
	return !os.IsNotExist(err)
}

// Connect is a wrapper around ssh.Dial using TCP.
//
// Although ssh.ClientConfig supports a zero timeout, i.e., no timeout, it's recommended to include
// a timeout to prevent Boomerang from hanging indefintely. A successful client connection may still get
// hung up by a downstream processes such as authentication, leaving Boomerang hanging.
//
// Retry specifies the number of times to retry the conection and wait specifies the number of seconds to wait
// before trying again. On each subsequent retry, up until the last, Boomerang will wait at most
// (ssh.ClientConfig.Timeout + wait)s.
//
// The deadline is the total number of seconds Boomerang will spend trying to connect.
func (m *Machine) Connect(conf *ssh.ClientConfig, retry, wait int64) (*ssh.Client, error) {

	if conf.Timeout == 0 {
		client, err := ssh.Dial("tcp", m.address(), conf)
		if err != nil {
			return nil, errors.Wrap(err, "could not establish machine connection")
		}
		return client, nil
	}

	deadline := conf.Timeout + (time.Duration(retry*wait) * time.Second) + (time.Duration(retry) * conf.Timeout)

	ctx, cancel := context.WithTimeout(context.Background(), deadline+(1*time.Second))
	defer cancel()

	ch := make(chan *ssh.Client, 1)
	ec := make(chan error, 1)

	go func(r int64) {
		for {
			client, err := ssh.Dial("tcp", m.address(), conf)
			if err != nil && r > 0 {
				time.Sleep(time.Duration(wait) * time.Second)
				r--
				continue
			}
			if err != nil {
				ec <- err
				return
			}

			ch <- client
			return
		}
	}(retry)

	select {
	case c := <-ch:
		return c, nil
	case e := <-ec:
		return nil, e
	case <-ctx.Done():
		return nil, errors.Errorf("Retried %v time(s) with a %v wait. No more retries!", retry, (time.Duration(wait) * time.Second))
	}
}

func (m *Machine) address() string { return m.HostName + ":" + m.Port }

// Run TODO comment
func (m *Machine) Run(st *State) *Machine {
	start := time.Now()

	if m.HostName == "127.0.0.1" || m.HostName == "localhost" {
		m.Connection = false
		m.RunLength = time.Since(start).Seconds()
		m.ConnectionErrors = []string{fmt.Sprintf("[%v] is not supported. Consider creating a feature proposal", m.HostName)}
		return m
	}

	if err := m.setSSHPort(); err != nil {
		m.Connection = false
		m.RunLength = time.Since(start).Seconds()
		m.ConnectionErrors = []string{fmt.Sprint(errors.Wrap(err, "failed port validation"))}
		return m
	}

	var hostChecking ssh.HostKeyCallback
	switch st.hostKeyCheck {
	case true:
		// Every client must provide a host key check.
		hostKey, err := checkHostKey(m.HostName, m.Port)
		if err != nil {
			m.Connection = false
			m.RunLength = time.Since(start).Seconds()
			m.ConnectionErrors = []string{fmt.Sprint(errors.Wrap(err, "failed host key check"))}
			return m
		}
		hostChecking = ssh.FixedHostKey(hostKey)
	case false:
		hostChecking = ssh.InsecureIgnoreHostKey()
	}

	conf := &ssh.ClientConfig{
		User:            m.Username,
		Auth:            []ssh.AuthMethod{st.auth},
		HostKeyCallback: hostChecking,
		Timeout:         time.Duration(st.connTimeout) * time.Second,
	}

	client, err := m.Connect(conf, st.retry, st.retryWait)
	if err != nil {
		m.Connection = false
		m.RunLength = time.Since(start).Seconds()
		m.ConnectionErrors = []string{fmt.Sprint(errors.Wrap(err, "failed client connection"))}
		return m
	}

	m.StreamData = executeCommands(client, st.commands)
	m.Connection = true
	m.RunLength = time.Since(start).Seconds()

	return m
}

func (m *Machine) setSSHPort() error {
	if m.Port == "" {
		m.Port = "22"
		return nil
	}

	i, err := strconv.Atoi(m.Port)
	if err != nil {
		return errors.Wrap(err, "converting port string to int failed")
	}
	if i < 1 || i > 65535 {
		return errors.Errorf("invalid port: [%v]", m.Port)
	}

	m.Port = strconv.Itoa(i)
	return nil
}

func executeCommands(client *ssh.Client, cs []command) []Stream {

	var out []Stream

	for _, c := range cs {

		sd := Stream{
			Name:         c.name,
			StreamErrors: make([]string, 0),
		}

		session, err := client.NewSession()
		if err != nil {
			sd.StreamErrors = append(sd.StreamErrors, fmt.Sprintf("error type=(%T): Failed to create NewSession: %v\n", errors.Cause(err), err))
			sd.ExitCode = -1
			out = append(out, sd)
			continue
		}
		defer session.Close()

		var stout, sterr bytes.Buffer
		session.Stdout = &stout
		session.Stderr = &sterr

		if err := session.Run(c.cmd); err != nil {
			switch e := err.(type) {
			case *ssh.ExitError:
				sd.StreamErrors = append(sd.StreamErrors, fmt.Sprintf("Command completed unsuccessfully: [%T]: %v", e, e.String()))
				sd.ExitCode = e.Waitmsg.ExitStatus()
			case *ssh.ExitMissingError:
				sd.StreamErrors = append(sd.StreamErrors, fmt.Sprintf("Exit code missing: %s", err))
				sd.ExitCode = -1
			default:
				sd.StreamErrors = append(sd.StreamErrors, fmt.Sprintf("Failed session Run: [%T]: %v", errors.Cause(err), err))
				sd.ExitCode = -1
			}
		}

		sd.Stdout = strings.TrimSpace(stout.String())
		sd.Stderr = strings.TrimSpace(sterr.String())

		out = append(out, sd)
	}

	return out
}
