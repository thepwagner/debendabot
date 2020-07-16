package manifest

import (
	"encoding/json"
	"io"
)

const LockFilename = "dpkg-lock.json"

type LockedPackage struct {
	Version      string `json:"version"`
	Architecture string `json:"architecture"`
	DebFilename  string `json:"filename"`
	DebHash      string `json:"filehash"`
}

type DpkgLockJSON struct {
	Image    string                        `json:"image"`
	Packages map[PackageName]LockedPackage `json:"packages"`
}

func ParseDpkgLockJSON(r io.Reader) (*DpkgLockJSON, error) {
	var d DpkgLockJSON
	if err := json.NewDecoder(r).Decode(&d); err != nil {
		return nil, err
	}
	return &d, nil
}
