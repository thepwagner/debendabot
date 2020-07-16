package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/thepwagner/debendabot/build"
	"github.com/thepwagner/debendabot/manifest"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Regenerate lock file",
	Long:  `Rebuild image and update lock file`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mf, err := parseManifest(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		return UpdateCommand(ctx, cmd, mf)
	},
}

func UpdateCommand(ctx context.Context, cmd *cobra.Command, mf *manifest.Manifest) error {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("opening docker client: %w", err)
	}
	defer cli.Close()
	b := build.NewBuilder(cli)

	// Calculate and write lockfile:
	lock, err := b.Lock(ctx, *mf)
	if err != nil {
		return fmt.Errorf("generating lockfile: %w", err)
	}

	dir, err := cmd.Flags().GetString(flagDir)
	if err != nil {
		return err
	}
	lfp, err := cmd.Flags().GetString(flagLockfilePath)
	if err != nil {
		return err
	}
	if err := writeLockfile(lock, filepath.Join(dir, lfp)); err != nil {
		return err
	}
	logrus.Info("wrote lockfile")
	return nil
}

func writeLockfile(lock *manifest.DpkgLockJSON, lockfilePath string) error {
	lf, err := os.OpenFile(lockfilePath, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("opening lockfile: %w", err)
	}
	defer lf.Close()

	if err := lf.Truncate(0); err != nil {
		return fmt.Errorf("truncating lockfile: %w", err)
	}
	encoder := json.NewEncoder(lf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(lock); err != nil {
		return fmt.Errorf("encoding lockfile: %w", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
