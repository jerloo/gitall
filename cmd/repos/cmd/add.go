/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"strings"

	"github.com/jerloo/repos"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a repository.",
	Run: func(cmd *cobra.Command, args []string) {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)
		workspace = strings.ReplaceAll(workspace, "$HOME", home)

		client, err := repos.NewRepoManager(workspace,
			repos.WithVerbose(verbose),
			repos.WithConfig(config),
			repos.WithCurrentWorkspace(),
		)
		cobra.CheckErr(err)

		err = client.Add(args[0])
		cobra.CheckErr(err)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// addCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// addCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
