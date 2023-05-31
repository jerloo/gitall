/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jerloo/repos"
	"github.com/spf13/cobra"

	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
)

var config *repos.ReposConfig

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "repos",
	Short: "Perform git operations of multiple repositories in batch.",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) {
	// 	cmd.Printf("version: %s\n", config.Version)
	// },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	defaultCfgFile := filepath.Join(homeDir, ".repos.yaml")

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", defaultCfgFile, "config file (default is $HOME/.repos.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Set verbose mode.")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile == "" {
		home, err := os.Getwd()
		cobra.CheckErr(err)
		cfgFile = filepath.Join(home, ".repos.yaml")
	}
	fmt.Println("Using config file:", cfgFile)
	viper.SetConfigFile(cfgFile)

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		// fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		err := viper.Unmarshal(&config)
		if err != nil {
			panic(err)
		}
		config.CfgFile = cfgFile
	}
	if config == nil {
		config = &repos.ReposConfig{
			CfgFile: cfgFile,
		}
	}
	config.Version = "1"
	config.Repos = make(map[string]*repos.RepoConfig)
}
