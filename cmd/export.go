package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/thepwagner/debendabot/build"
	"github.com/thepwagner/debendabot/manifest"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export image to container",
	Long:  `Build docker container from manifest`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mf, err := parseManifest(cmd)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		return ExportCommand(ctx, cmd, *mf)
	},
}

const (
	flagDocker = "docker"
	flagExt4   = "ext4"
)

func ExportCommand(ctx context.Context, cmd *cobra.Command, mf manifest.Manifest) error {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("opening docker client: %w", err)
	}
	defer cli.Close()
	b := build.NewBuilder(cli)

	dir, err := cmd.Flags().GetString(flagDir)
	if err != nil {
		return err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return err
	}

	if err := b.Build(ctx, mf); err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	// Start container to export chroot to tarball:
	if err := exportImageTarball(ctx, cli, mf, dir); err != nil {
		return err
	}

	toDocker, err := cmd.Flags().GetBool(flagDocker)
	if err != nil {
		return err
	}
	if toDocker {
		imageFile, err := os.Open(filepath.Join(dir, "image.tar"))
		if err != nil {
			return err
		}
		_, err = cli.ImageImport(ctx, types.ImageImportSource{
			Source:     imageFile,
			SourceName: "-",
		}, mf.DpkgJSON.Image, types.ImageImportOptions{})
		if err != nil {
			return fmt.Errorf("importing image: %w", err)
		}
		logrus.WithField("image", mf.DpkgJSON.Image).Info("docker import complete")
	}
	return nil
}

func exportImageTarball(ctx context.Context, cli *client.Client, mf manifest.Manifest, dir string) error {
	ctr, err := cli.ContainerCreate(ctx, &container.Config{
		Image:        build.BuildImage(mf),
		AttachStdout: true,
		Entrypoint: []string{
			"sh", "-c", "tar -C $ROOTFS_PATH -c . -f /out/image.tar",
		},
	}, &container.HostConfig{
		AutoRemove: true,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: dir,
				Target: "/out",
			},
		},
	}, nil, "")
	if err != nil {
		return fmt.Errorf("creating export container: %w", err)
	}
	logrus.WithField("container_id", ctr.ID).Debug("created export container")
	statusCh, errCh := cli.ContainerWait(ctx, ctr.ID, container.WaitConditionNextExit)
	if err := cli.ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("starting epxort container: %w", err)
	}
	logrus.WithField("container_id", ctr.ID).Debug("started export container")
	select {
	case err := <-errCh:
		if err != nil {
			logrus.WithError(err).Warn("export container error")
		}
	case s := <-statusCh:
		logrus.WithField("status", s.StatusCode).Debug("export container finished")
	}
	return nil
}

func init() {
	exportCmd.Flags().Bool(flagDocker, true, "export to docker")
	rootCmd.AddCommand(exportCmd)
}
