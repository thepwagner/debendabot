package build

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strings"

	docker "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
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
	return b.build(ctx, dpkgJSON, "", dpkgJSON.Image)
}

func (b *Builder) build(ctx context.Context, dpkgJSON *manifest.DpkgJSON, target string, tag string) error {
	logger := logrus.WithField("image", dpkgJSON.Image)

	// Generate Dockerfile and prepare context:
	dockerfile, err := genDockerfile(dpkgJSON)
	if err != nil {
		return fmt.Errorf("generating dockerfile: %w", err)
	}
	logger.WithField("dockerfile", dockerfile).Debug("generated dockerfile")
	contextTar, err := buildContext(dockerfile)
	if err != nil {
		return fmt.Errorf("preparing build context: %w", err)
	}

	// Perform the build:
	build, err := b.docker.ImageBuild(ctx, contextTar, docker.ImageBuildOptions{
		Dockerfile: "/Dockerfile",
		Tags:       []string{tag},
		Target:     target,
	})
	if err != nil {
		return fmt.Errorf("building image: %w", err)
	}
	defer build.Body.Close()

	var buildOutput bytes.Buffer
	if err := jsonmessage.DisplayJSONMessagesStream(build.Body, &buildOutput, 0, false, nil); err != nil {
		return fmt.Errorf("reading build output: %w", err)
	}

	// Spam:
	logger.Info("completed build")
	for _, l := range strings.Split(buildOutput.String(), "\n") {
		logger.WithField("line", strings.TrimSpace(l)).Debug("build output")
	}
	return nil
}

var packageLine = regexp.MustCompile("(?P<package>[^/]+)/(?P<release>[^ ]+) (?P<version>[^ ]+) (?P<arch>[^ ]+) (?P<meta>[^ ]+)")

func (b *Builder) Lock(ctx context.Context, dpkgJSON *manifest.DpkgJSON) (*manifest.DpkgLockJSON, error) {
	manifestImage := fmt.Sprintf("debendabot-manifest/%s", dpkgJSON.Image)
	if err := b.build(ctx, dpkgJSON, "manifest", manifestImage); err != nil {
		return nil, fmt.Errorf("rebuilding manifest: %w", err)
	}

	// Pin the docker parent to a SHA:
	image, _, err := b.docker.ImageInspectWithRaw(ctx, manifestImage)
	if err != nil {
		return nil, fmt.Errorf("querying manifest image: %w", err)
	}
	pinnedImage := fmt.Sprintf("debian@%s", image.Parent)
	dpkgLock := &manifest.DpkgLockJSON{
		Image: pinnedImage,
	}

	// Extract manifest file:
	ctr, err := b.docker.ContainerCreate(ctx, &container.Config{
		Image: manifestImage,
	}, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("creating manifest image: %w", err)
	}
	defer func() {
		err := b.docker.ContainerRemove(ctx, ctr.ID, docker.ContainerRemoveOptions{Force: true})
		if err != nil {
			logrus.WithError(err).Warn("error removing manifest container")
		}
	}()

	aptInstalled, err := b.readFile(ctx, ctr.ID, "/apt-installed.txt")
	if err != nil {
		return nil, err
	}
	pkgList := strings.Split(string(aptInstalled), "\n")

	dpkgLock.Packages = make(map[manifest.PackageName]manifest.LockedPackage, len(pkgList))
	for _, installedPackage := range pkgList {
		if installedPackage == "" {
			continue
		}
		match := packageLine.FindStringSubmatch(installedPackage)
		if len(match) == 0 {
			logrus.WithField("line", installedPackage).Warn("unmatched package line")
			continue
		}

		var pkg manifest.PackageName
		var lock manifest.LockedPackage
		for i, name := range packageLine.SubexpNames() {
			switch name {
			case "package":
				pkg = manifest.PackageName(match[i])
			case "version":
				lock.Version = match[i]
			case "arch":
				lock.Architecture = match[i]
			}
		}
		dpkgLock.Packages[pkg] = lock
	}
	return dpkgLock, nil
}

func (b *Builder) readFile(ctx context.Context, containerID string, path string) ([]byte, error) {
	copied, _, err := b.docker.CopyFromContainer(ctx, containerID, path)
	if err != nil {
		return nil, fmt.Errorf("copying container file: %w", err)
	}
	defer copied.Close()

	tr := tar.NewReader(copied)
	// Discard header:
	if _, err := tr.Next(); err != nil {
		return nil, fmt.Errorf("reading copied tar: %w", err)
	}
	return ioutil.ReadAll(tr)
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
