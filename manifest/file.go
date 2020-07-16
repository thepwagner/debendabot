package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func ParseManifests(dir, manifestPath, lockfilePath string) (*DpkgJSON, *DpkgLockJSON, error) {
	mfp := filepath.Join(dir, manifestPath)
	mf, err := os.Open(mfp)
	if err != nil {
		return nil, nil, fmt.Errorf("opening %q: %w", mfp, err)
	}
	defer mf.Close()
	dpkgJSON, err := ParseDpkgJSON(mf)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %q: %w", mfp, err)
	}

	lfp := filepath.Join(dir, lockfilePath)
	lf, err := os.Open(lfp)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return dpkgJSON, nil, nil
		}
		return nil, nil, fmt.Errorf("opening %q: %w", lfp, err)
	}
	defer lf.Close()

	dpkgLockJSON, err := ParseDpkgLockJSON(lf)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %q: %w", lfp, err)
	}
	return dpkgJSON, dpkgLockJSON, nil
}
