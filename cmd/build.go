package cmd

import (
	"fmt"

	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build container",
	Long:  `Build docker container from manifest`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logrus.Debug("build")
		return nil
	},
}

func BuildCommand() error {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("opening docker client: %w", err)
	}
	defer cli.Close()

	// TODO: build

	return nil
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
