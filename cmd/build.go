package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"github.com/thepwagner/debendabot/build"
	"github.com/thepwagner/debendabot/manifest"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build image",
	Long:  `Assemble image from manifest`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mf, err := parseManifest(cmd)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		return BuildCommand(ctx, mf)
	},
}

func BuildCommand(ctx context.Context, mf *manifest.Manifest) error {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("opening docker client: %w", err)
	}
	defer cli.Close()
	b := build.NewBuilder(cli)

	if err := b.Build(ctx, *mf); err != nil {
		return fmt.Errorf("building image: %w", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
