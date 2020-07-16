package build

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	docker "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/sirupsen/logrus"
	"github.com/thepwagner/debendabot/manifest"
)

type Builder struct {
	docker *client.Client
}

func NewBuilder(docker *client.Client) *Builder {
	return &Builder{docker: docker}
}

func (b *Builder) Build(ctx context.Context, dpkgJSON *manifest.DpkgJSON) error {
	logger := logrus.WithField("image", dpkgJSON.Image)

	// Generate Dockerfile and prepare context:
	dockerfile, err := genDockerfile(dpkgJSON)
	if err != nil {
		return fmt.Errorf("generating dockerfile: %w", err)
	}
	fmt.Println(dockerfile)
	logger.WithField("dockerfile", dockerfile).Debug("generated dockerfile")
	contextTar, err := buildContext(dockerfile)
	if err != nil {
		return fmt.Errorf("preparing build context: %w", err)
	}

	// Perform the build:
	build, err := b.docker.ImageBuild(ctx, contextTar, docker.ImageBuildOptions{
		SuppressOutput: false,
		Dockerfile:     "/Dockerfile",
		Tags:           []string{dpkgJSON.Image},
	})
	if err != nil {
		return fmt.Errorf("building image: %w", err)
	}
	defer build.Body.Close()

	var buildOutput bytes.Buffer
	if err := jsonmessage.DisplayJSONMessagesStream(build.Body, &buildOutput, 0, false, nil); err != nil {
		fmt.Println(buildOutput.String())
		return fmt.Errorf("reading build output: %w", err)
	}

	// Spam:
	logger.Info("completed build")
	for _, l := range strings.Split(buildOutput.String(), "\n") {
		logger.WithField("line", strings.TrimSpace(l)).Debug("build output")
	}
	return nil
}

func buildContext(dockerfile string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// /Dockerfile
	th := &tar.Header{
		Name: "Dockerfile",
		Mode: 0400, // don't trust anybody
		Size: int64(len(dockerfile)),
	}
	if err := tw.WriteHeader(th); err != nil {
		return nil, fmt.Errorf("writing tar header: %w", err)
	}
	_, err := tw.Write([]byte(dockerfile))
	if err != nil {
		return nil, fmt.Errorf("writing dockerfile: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("closing tar: %w", err)
	}

	return &buf, nil
}
