package manifest

const LockFilename = "dpkg-lock.json"

type LockedPackage struct {
	Version      string `json:"version"`
	Architecture string `json:"architecture"`
	// TODO: hash the deb?
}

type DpkgLockJSON struct {
	Image    string                        `json:"image"`
	Packages map[PackageName]LockedPackage `json:"packages"`
}
