package main

import (
	"os/user"

	"github.com/spf13/cobra"
)

var prefix string
var sshUser string
var identityFile string
var jumphost string
var filters string

var rootCmd = &cobra.Command{
	Use: "generate-ssh-configs",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		checkErr(err)
	},
}

var awsCmd = &cobra.Command{
	Use: "aws",
	Run: func(cmd *cobra.Command, args []string) {
		if prefix != "" {
			prefix = prefix + "-"
		}
		generateAWS(prefix)
	},
}

var digitalOceanCmd = &cobra.Command{
	Use: "digital-ocean",
	Run: func(cmd *cobra.Command, args []string) {
		if prefix != "" {
			prefix = prefix + "-"
		}
		generateDigitalOcean(prefix)
	},
}

func requirePrefix(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&prefix,
		"prefix",
		"",
		"The prefix thats used in the ssh file for this group",
	)
}

func userFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&sshUser,
		"user",
		"",
		"The ssh user",
	)
}

func identityFileFlag(cmd *cobra.Command) {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	defaultIdentityFile := usr.HomeDir + "/.ssh/id_rsa"
	cmd.Flags().StringVar(
		&identityFile,
		"identityFile",
		defaultIdentityFile,
		"",
	)
}

func cmdInit() {
	requirePrefix(awsCmd)
	requirePrefix(digitalOceanCmd)

	userFlag(awsCmd)
	userFlag(digitalOceanCmd)
	identityFileFlag(awsCmd)
	awsCmd.Flags().StringVar(
		&jumphost,
		"jumphost",
		"",
		"The jumphost",
	)
	awsCmd.Flags().StringVar(
		&filters,
		"filters",
		"",
		"AWS instance filters",
	)

	rootCmd.AddCommand(awsCmd)
	rootCmd.AddCommand(digitalOceanCmd)
}
