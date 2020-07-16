package cmd

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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
		return ExportCommand(ctx, *mf)
	},
}

func ExportCommand(ctx context.Context, mf manifest.Manifest) error {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("opening docker client: %w", err)
	}
	defer cli.Close()
	b := build.NewBuilder(cli)

	if err := b.Build(ctx, mf); err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	// TODO: start container to export chroot
	ctr, err := cli.ContainerCreate(ctx, &container.Config{
		Image:        build.BuildImage(mf),
		AttachStdout: true,
		AttachStderr: true,
		Entrypoint: []string{
			"sh", "-c", "tar -vv -C $ROOTFS_PATH -c .",
		},
	}, &container.HostConfig{
		AutoRemove: true,
	}, nil, "")
	if err != nil {
		return fmt.Errorf("creating export container: %w", err)
	}
	logrus.WithField("container_id", ctr.ID).Debug("created export container")

	statusCh, errCh := cli.ContainerWait(ctx, ctr.ID, container.WaitConditionNotRunning)
	go func() {
		select {
		case err := <-errCh:
			if err != nil {
				logrus.WithError(err).Warn("export container error")
			}
		case s := <-statusCh:
			logrus.WithField("status", s.StatusCode).Debug("export container finished")
		}
	}()

	attach, err := cli.ContainerAttach(ctx, ctr.ID, types.ContainerAttachOptions{
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("attaching to export container: %w", err)
	}
	defer attach.Close()
	logrus.WithField("container_id", ctr.ID).Debug("attached to export container")

	pipeR, pipeW := io.Pipe()
	go func() {
		tee := io.MultiWriter(pipeW, os.Stdout)
		if _, err := stdcopy.StdCopy(tee, ioutil.Discard, attach.Reader); err != nil {
			logrus.WithError(err).Warn("copying container import to export")
		}
	}()
	if err := cli.ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("starting epxort container: %w", err)
	}
	logrus.WithField("container_id", ctr.ID).Debug("started export container")

	_, err = cli.ImageImport(ctx, types.ImageImportSource{
		Source:     pipeR,
		SourceName: "-",
	}, mf.DpkgJSON.Image, types.ImageImportOptions{})
	if err != nil {
		return fmt.Errorf("importing image: %w", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(exportCmd)
}
