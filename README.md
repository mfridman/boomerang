`boomerang` executes a list of commands on many machines, concurrently, and returns a JSON file. Written entirely in Go.

## Brief

The project needs a bit more work and the API may change. But the code base is workable, e.g., able to execute dozens of commands on hundreds of machines concurrently. See [todo list](#to-do).

One could run `boomerang` as a cron job to gather data and feed the resulting JSON file to a downstream parser.

[options](#user-options) and [commands](#commands) are read from a single local config file.

A typical project layout:

Note, `boomerang` will create its own `raw` directory.

```shell
.
├── config
├── boomerang
└── raw
    ├── raw_20170506_173824.json
    ├── raw_20170506_182750.json
```

## Installation

This project aims to be a cli tool, hence the package layout.

    go get -u github.com/mfridman/boomerang/cmd/boomerang

## Config file

File name should be config.yml and be located in the same directory as `boomerang`. Can override default via `--c` flag with a custom path and name.

config file consists of options and commands, all within a single file.

### User options

- `inventory` is mandatory, [see below](#inventory)
- should be explicit about authentication method. `agent`, `key` and `password` are supported.
    - if using auth=password, must supply `SSHpassword` option
    - if using auth=key, must supply `privKeyLocation` option
    - if using auth=agent, can supply custom env variable via `agentSSHAuth`, otherwise defaults to `SSH_AUTH_SOCK`

Full list of user options can be found [here](#available-options)

Example:

```yaml
inventory: my_machines.json # or http://10.0.0.6/api/v1/machines
auth: key
privKeyLocation: /Users/machine2b/.ssh/google_compute_engine
connTimeout: 10
keepLatestFile: true
indentJSON: true
```

### Commands

- Commands are run sequentially, by design, and must be specified as a YAML list of key/value pairs.

```yaml
commands:
    - uptime: /usr/bin/uptime
    - ubuntu_version: 
```

Sample output:

```json
stream_data: [
    {
        "name": "uptime",
        "stdout": "23:45:20 up 128 days, 12:50,  0 users,  load average: 0.08, 0.13, 0.09",
        "stderr": "",
        "exit_code": 0,
        "stream_errors": []
    },
    {
        "name": "ubuntu_version",
        "stdout": "Description:\tUbuntu 16.04.2 LTS",
        "stderr": "",
        "exit_code": 0,
        "stream_errors": []
    }
]
```

## Inventory

`boomerang` builds a list of machines as specified by `inventory` (a mandatory [config file](#config-file) option), can be:

1.  a file in the same directory as `boomerang` 
    - my_machines.json
2.  an absolute path
    - /etc/boomerang/inventory.json
3.  a network address
    - https://example.com/dev_servers/api or http://10.0.0.6/api/v1/machines  

Inventory is an array of machine objects, where each machine object contains:

- `username` and `hostname`, both are mandatory fields
- `ssh_port` accepts 1-65535; blank defaults to port 22
- `extras` is optional and will be written out as is to final JSON. Can be used to record machine-specific metadata, e.g., name, location, id.

```json
[
    {
        "username": "me",
        "hostname": "upspin.mfridman.com",
    },
    {
        "username": "user",
        "hostname": "192.168.10.53",
        "ssh_port": "41622",
        "extras": {
            "name": "ubuntu16-media",
            "location": "home"
        }
    }
]
```

# Common issues

## known hosts

It's good practice to check the machine you're connecting to _is_ the intended machine. Which is why the first time you connect to a host via SSH you get the following:

```
The authenticity of host '136.138.52.76 (136.138.52.76)' can't be established.
ECDSA key fingerprint is SHA256:a3FBPiAznngxKS9XGqua9TbVa5aASD/NvjOaZQUxkLM.
Are you sure you want to continue connecting (yes/no)? 
```

On the server side, if you run `ssh-keygen -E sha256 -l -f /etc/ssh/ssh_host_ecdsa_key` the fingerprint will match up to the above:

```
256 SHA256:a3FBPiAznngxKS9XGqua9TbVa5aASD/NvjOaZQUxkLM
```

With that understanding, you have 2 options:

1.  add the machine hostkey to your known_hosts file, usually $HOME/.ssh/known_hosts
2.  add `host_key_check: false` to config file, enabling `boomerang` to bypass hostkey checking. __Although this works, be warned this is insecure. AVOID using this in production!__

### Available options

| Name | Type | Default | example or description |
|---|---|---|---|
|inventory|string||my_machines.json, http://10.0.0.6/api/v1/machines
|auth|string||key\|agent\|password|
|privKeyLocation|string||/home/user/id\_dsa|
|password|string||"superS3cret{r1ght}?;". If possible, use key or agent instead|
|agentSSHAuth|string|SSH_AUTH_SOCK||
|__OPTIONAL__||||
|connTimeout|int|10||
|machineType|string|""|displays in metadata|
|hostKeyCheck|bool|true|false\|true (see [known hosts](#known-hosts) section below)|
|keepLatestFile|bool|false|false\|true, **Warning** if true will delete all existing .json files in raw folder and keep latest .json file only|
|indentJSON|bool|true|true\|false, if true will indent resulting JSON file|
|prefixJSON|string|raw|user can specify JSON filename prefix. json_prefix will be suffixed with `_yyyymmdd_hhmmss.json`. E.g, raw_20170506_173824.json|
|retry|int|1||
|retryWait|int|15||

# To Do

- [ ] move todo list to Github issues
- [ ] merge .go files in pkg
- [ ] write tests
- [ ] decide on exported APIs (if any)
- [ ] add flag options for mandatory config file options
- [ ] allow custom known\_hosts, otherwise default to .ssh/known_hosts
- [ ] standardize error messages across all packages, more user friendly
- [ ] add flag enabling writing to console instead of just to a JSON
- [ ] consider adding sudo support
- [ ] add option to stop further execution on given machine upon a single command failure
- [ ] enable reading multiple config files from a directory. <- possible to have varying machine types that require a different set of commands?
- [ ] program needs a clean exit in the event something goes wrong