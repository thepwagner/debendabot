package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"github.com/thepwagner/debendabot/build"
	"github.com/thepwagner/debendabot/manifest"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build container",
	Long:  `Build docker container from manifest`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := cmd.Flags().GetString(flagDir)
		if err != nil {
			return err
		}
		mfp, err := cmd.Flags().GetString(flagManifestPath)
		if err != nil {
			return err
		}
		lfp, err := cmd.Flags().GetString(flagLockfilePath)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		return BuildCommand(ctx, dir, mfp,lfp)
	},
}

const (
	flagDir          = "dir"
	flagManifestPath = "manifest"
	flagLockfilePath = "lockfile"
)

func BuildCommand(ctx context.Context, dir , manifestPath, lockfilePath string) error {
	mf, err := parseDpkgJSON(filepath.Join(dir, manifestPath))
	if err != nil {
		return err
	}
	// TODO: load lockfile

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("opening docker client: %w", err)
	}
	defer cli.Close()
	b := build.NewBuilder(cli)

	// Calculate and write lockfile:
	lock, err := b.Lock(ctx, mf)
	if err != nil {
		return fmt.Errorf("generating lockfile: %w", err)
	}
	if err := writeLockfile(lock, filepath.Join(dir, lockfilePath)); err != nil {
		return err
	}

	return nil
}

func parseDpkgJSON(path string) (*manifest.DpkgJSON, error) {
	mf, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %q: %w", path, err)
	}
	defer mf.Close()
	return manifest.ParseDpkgJSON(mf)
}

func writeLockfile(lock *manifest.DpkgLockJSON, lockfilePath string) error {
	lf, err := os.OpenFile(lockfilePath, os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("opening lockfile: %w", err)
	}
	encoder := json.NewEncoder(lf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(lock); err != nil {
		return fmt.Errorf("encoding lockfile: %w", err)
	}
	return nil
}

func init() {
	buildCmd.Flags().StringP(flagDir, "d", ".", "Directory of manifest")
	buildCmd.Flags().StringP(flagManifestPath, "m", manifest.Filename, "Manifest filename")
	buildCmd.Flags().StringP(flagLockfilePath, "l", manifest.LockFilename, "Lockfile filename")
	rootCmd.AddCommand(buildCmd)
}
