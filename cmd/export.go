package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	tarImageName = "image.tar"
	extImageName = "image.ext4"
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

	if err := dockerExport(ctx, cmd, cli, dir, mf); err != nil {
		return err
	}

	if err := ext4Export(ctx, cmd, cli, dir, mf); err != nil {
		return err
	}
	return nil
}

func dockerExport(ctx context.Context, cmd *cobra.Command, cli *client.Client, dir string, mf manifest.Manifest) error {
	toDocker, err := cmd.Flags().GetBool(flagDocker)
	if err != nil {
		return err
	}
	if !toDocker {
		return nil
	}

	imageFile, err := os.Open(filepath.Join(dir, tarImageName))
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
	return nil
}

func ext4Export(ctx context.Context, cmd *cobra.Command, cli *client.Client, dir string, mf manifest.Manifest) error {
	toExt4, err := cmd.Flags().GetBool(flagExt4)
	if err != nil {
		return err
	}
	if !toExt4 {
		return nil
	}

	ctr, err := cli.ContainerCreate(ctx, &container.Config{
		Image: build.BuildImage(mf),
		Entrypoint: []string{
			"sh", "-c",
			strings.Join([]string{
				fmt.Sprintf("truncate -s 0M /out/%s", extImageName),
				fmt.Sprintf("truncate -s 512M /out/%s", extImageName),
				fmt.Sprintf("mkfs.ext4 -m0 /out/%s", extImageName),
				fmt.Sprintf("mount /out/%s /mnt", extImageName),
				"tar -C /mnt -xvvf /out/image.tar",
			}, " && ") +
				"; umount -f /mnt",
		},
	}, &container.HostConfig{
		//AutoRemove: true,
		Privileged: true,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: dir,
				Target: "/out",
			},
		},
	}, nil, "")
	if err != nil {
		return fmt.Errorf("creating ext4 conversion container: %w", err)
	}
	logrus.WithField("container_id", ctr.ID).Debug("created ext4 conversion container")
	statusCh, errCh := cli.ContainerWait(ctx, ctr.ID, container.WaitConditionNextExit)
	if err := cli.ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("starting ext4 conversion container: %w", err)
	}
	logrus.WithField("container_id", ctr.ID).Debug("started ext4 conversion container")
	select {
	case err := <-errCh:
		if err != nil {
			logrus.WithError(err).Warn("ext4 conversion container error")
		}
	case s := <-statusCh:
		logrus.WithField("status", s.StatusCode).Debug("ext4 conversion container finished")
	}

	logrus.WithField("image", extImageName).Info("ext4 conversion complete")
	return nil
}

func exportImageTarball(ctx context.Context, cli *client.Client, mf manifest.Manifest, dir string) error {
	ctr, err := cli.ContainerCreate(ctx, &container.Config{
		Image: build.BuildImage(mf),
		Entrypoint: []string{
			"sh", "-c", fmt.Sprintf("tar -C $ROOTFS_PATH -c . -f /out/%s", tarImageName),
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
	exportCmd.Flags().Bool(flagExt4, false, "export as ext4 filesystem")
	rootCmd.AddCommand(exportCmd)
}
