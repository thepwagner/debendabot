package manifest

import (
	"encoding/json"
	"io"
)

const Filename = "dpkg.json"

type PackageName string
type PackageVersion string

type DpkgJSON struct {
	Image    string                         `json:"image"`
	Distro   string                         `json:"distro"`
	Packages map[PackageName]PackageVersion `json:"packages"`
	// TODO: repositories, keys?
}

func ParseDpkgJSON(r io.Reader) (*DpkgJSON, error) {
	var d DpkgJSON
	if err := json.NewDecoder(r).Decode(&d); err != nil {
		return nil, err
	}
	return &d, nil
}
