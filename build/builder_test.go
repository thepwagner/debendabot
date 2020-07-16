package build_test

import (
	"context"
	"testing"

	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/debendabot/build"
	"github.com/thepwagner/debendabot/manifest"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func TestBuilder_Build(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	require.NoError(t, err)
	defer cli.Close()

	b := build.NewBuilder(cli)

	ctx := context.Background()
	m := &manifest.DpkgJSON{
		Image:  "test",
		Distro: "buster",
		Packages: map[manifest.PackageName]manifest.PackageVersion{
			"bash": "stable",
		},
	}
	err = b.Build(ctx, m)
	require.NoError(t, err)
}

func TestBuilder_Lock(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	require.NoError(t, err)
	defer cli.Close()

	b := build.NewBuilder(cli)

	ctx := context.Background()
	m := &manifest.DpkgJSON{
		Image:  "test",
		Distro: "buster",
		Packages: map[manifest.PackageName]manifest.PackageVersion{
			"bash": "stable",
		},
	}
	dpkgLock, err := b.Lock(ctx, m)
	require.NoError(t, err)

	assert.NotEmpty(t, dpkgLock.Image)
	assert.NotEmpty(t, dpkgLock.Packages)
}