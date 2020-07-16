package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Manifest struct {
	DpkgJSON     DpkgJSON
	DpkgLockJSON *DpkgLockJSON
}

func ParseManifest(dir, manifestPath, lockfilePath string) (*Manifest, error) {
	mfp := filepath.Join(dir, manifestPath)
	mf, err := os.Open(mfp)
	if err != nil {
		return nil, fmt.Errorf("opening %q: %w", mfp, err)
	}
	defer mf.Close()
	dpkgJSON, err := ParseDpkgJSON(mf)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", mfp, err)
	}

	lfp := filepath.Join(dir, lockfilePath)
	lf, err := os.Open(lfp)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Return without lockfile:
			return &Manifest{DpkgJSON: *dpkgJSON}, nil
		}
		return nil, fmt.Errorf("opening %q: %w", lfp, err)
	}
	defer lf.Close()

	dpkgLockJSON, err := ParseDpkgLockJSON(lf)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", lfp, err)
	}
	return &Manifest{
		DpkgJSON:     *dpkgJSON,
		DpkgLockJSON: dpkgLockJSON,
	}, nil
}

func (m *Manifest) PackageCount() int {
	if m == nil {
		return 0
	}
	return len(m.DpkgJSON.Packages)
}

func (m *Manifest) LockedPackageCount() int {
	if m == nil || m.DpkgLockJSON == nil {
		return 0
	}
	return len(m.DpkgLockJSON.Packages)
}
