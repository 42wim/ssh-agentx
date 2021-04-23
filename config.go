package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

func (s *SSHAgent) parseConfig() (*viper.Viper, error) {
	v := viper.New()

	cfgPath, err := os.UserConfigDir()
	if err != nil {
		return v, err
	}

	v.AddConfigPath(".")
	v.AddConfigPath(filepath.Join(cfgPath, agentName))
	v.SetConfigName(agentName)

	if err := v.ReadInConfig(); err != nil {
		return v, err
	}
	/*
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				return v,fmt.Errorf("config file not found in %s, continuing as normal ssh-agent", err)
			} else {
				return v, fmt.Errorf("error reading config file %s", err)
		}
	*/

	// reload config on file changes
	v.WatchConfig()

	return v, nil
}
