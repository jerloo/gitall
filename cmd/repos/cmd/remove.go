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

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a repository.",
	Run: func(cmd *cobra.Command, args []string) {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)
		workspace = strings.ReplaceAll(workspace, "$HOME", home)

		client, err := repos.NewRepoManager(workspace,
			repos.WithVerbose(verbose),
			repos.WithConfig(config),
		)
		cobra.CheckErr(err)

		err = client.Remove(args[0])
		cobra.CheckErr(err)
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// removeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// removeCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
