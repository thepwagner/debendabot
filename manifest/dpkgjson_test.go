package manifest_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/debendabot/manifest"
)

const examplesDir = "../examples"

func TestParseDpkgJSON(t *testing.T) {
	// where we're going, we don't need test tables - doc brown
	examples, err := ioutil.ReadDir(examplesDir)
	require.NoError(t, err)
	require.NotEmpty(t, examples, "examples not found")

	for _, e := range examples {
		t.Run(e.Name(), func(t *testing.T) {
			path := filepath.Join(examplesDir, e.Name(), manifest.Filename)
			f, err := os.Open(path)
			require.NoError(t, err)
			defer f.Close()

			m, err := manifest.ParseDpkgJSON(f)
			require.NoError(t, err)
			t.Logf("%+v", m)
			assert.NotEqual(t, "", m.Image)
			assert.NotEqual(t, "", m.Distro)
			assert.NotEmpty(t, m.Packages)
		})
	}
}
