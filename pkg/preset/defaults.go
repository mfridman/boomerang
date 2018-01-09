package preset

import "github.com/spf13/viper"

// setViperDefaults sets sensible defaults used throughout the program, and
// registers aliases where necessary.
func setViperDefaults() {
	viper.RegisterAlias("Config", "config")

	// mandatory
	viper.RegisterAlias("Inventory", "inventory")
	viper.RegisterAlias("Auth", "auth")

	// mandatory if using auth=password
	viper.RegisterAlias("Password", "password")
	// mandatory if using auth=key
	viper.RegisterAlias("PrivKey", "key_location")
	// optional if using auth=agent, defaults to SSH_AUTH_SOCK
	viper.RegisterAlias("AgentSSHAuth", "agent_ssh_auth")
	viper.SetDefault("AgentSSHAuth", "SSH_AUTH_SOCK")

	// optional, setup program defaults

	viper.RegisterAlias("HostKeyCheck", "host_key_check")
	viper.SetDefault("HostKeyCheck", true)

	viper.RegisterAlias("ConnTimeout", "connection_timeout")
	viper.SetDefault("ConnTimeout", 10)

	viper.RegisterAlias("Type", "type")
	viper.SetDefault("Type", "")

	viper.RegisterAlias("KeepLatestFileOnly", "keep_latest_file_only")
	viper.SetDefault("KeepLatestFileOnly", false)

	viper.RegisterAlias("IndentJSON", "indent_json")
	viper.SetDefault("IndentJSON", true)

	viper.RegisterAlias("PrefixJSON", "json_prefix")
	viper.SetDefault("PrefixJSON", "raw")

	viper.RegisterAlias("Retry", "retry")
	viper.SetDefault("Retry", 1)

	viper.RegisterAlias("RetryWait", "retry_wait")
	viper.SetDefault("RetryWait", 15)

}
