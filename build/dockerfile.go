package build

import (
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/thepwagner/debendabot/manifest"
)

var dockerfileTemplate = template.Must(template.New("dockerfile").Parse(`
FROM {{.BaseImage}} AS base

FROM base AS sources
{{if .Proxy}}
ENV http_proxy={{.Proxy}}
{{end}}
{{/* 
	TODO: setup repos, install keys
	install gnupg+apt-transport-https if missing
*/}}
RUN apt-get update

FROM sources AS build
ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
  apt-get install -y \
   --no-install-recommends \
   debootstrap

ENV ROOTFS_PATH=/rootfs
RUN debootstrap \
  --arch amd64 \
  --variant=minbase \
  {{.Distro}} \
  ${ROOTFS_PATH} http://cdn-fastly.deb.debian.org/debian 

{{ if .LockedPackages }}
RUN chroot $ROOTFS_PATH sh -c "apt-get install -y --no-install-recommends \
{{ range $packageSpec := .LockedPackageSpecs }}
	{{$packageSpec}} \
{{ end }}
  && apt-mark auto \
{{ range $packageSpec := .LockedPackages }}
	{{$packageSpec}} \
{{ end }}
  && true"
{{ end }}
RUN chroot $ROOTFS_PATH sh -c "apt-get install -y --no-install-recommends \
{{ range $packageSpec := .PackageSpecs }}
	{{$packageSpec}} \
{{ end }}
  && true"

{{ if .LockedPackages }}
RUN chroot $ROOTFS_PATH apt-get --purge -y autoremove
{{ end }}


{{ if .DebHashes }}
RUN cd $ROOTFS_PATH/var/cache/apt/archives && \
  rm -f SHASUMS \
{{ range $debHash := .DebHashes }}
  && echo "{{$debHash}}" >> SHASUMS \
{{ end }}
  && sha512sum -c SHASUMS \
  && rm -f SHASUMS
{{ end }}

FROM build AS manifest
RUN chroot $ROOTFS_PATH apt list --installed -qq | tee /apt-installed.txt
RUN cd $ROOTFS_PATH/var/cache/apt/archives && sha512sum *.deb | tee /deb-hashes.txt

FROM build
RUN rm -Rf $ROOTFS_PATH/var/cache/apt/* $ROOTFS_PATH/var/lib/apt/lists/*
RUN rm -Rf $ROOTFS_PATH/usr/share/man/*
RUN find $ROOTFS_PATH/var/log -type f -exec truncate -s0 {} \;
CMD ["/usr/bin/bash"]
{{if .Proxy}}
ENV http_proxy=
{{end}}
`))

type dockerfileTemplateParams struct {
	Distro             string
	BaseImage          string
	LockedPackageSpecs []string
	LockedPackages     []string
	PackageSpecs       []string
	Proxy              string
	DebHashes          []string
}

func genDockerfile(mf manifest.Manifest) (string, error) {
	var buf strings.Builder

	p := dockerfileTemplateParams{
		Distro:    mf.DpkgJSON.Distro,
		BaseImage: baseImage(mf),
		// FIXME: don't hardcode my apt-cacher
		Proxy: "http://172.17.0.1:3142",
	}

	// Build package specs from dpkg.json:
	for name, version := range mf.DpkgJSON.Packages {
		switch version {
		case "stable", "unstable", "testing":
			p.PackageSpecs = append(p.PackageSpecs, fmt.Sprintf("%s/%s", name, version))
		default:
			// XXX: this requires an exact match - could we filter candidates against semver?
			// not all packages _are_ semver; some library can handle that? :crossed_fingers:
			p.PackageSpecs = append(p.PackageSpecs, fmt.Sprintf("%s=%s", name, version))
		}
	}
	sort.Strings(p.PackageSpecs)

	if mf.DpkgLockJSON != nil {
		for name, lock := range mf.DpkgLockJSON.Packages {
			p.LockedPackageSpecs = append(p.LockedPackageSpecs, fmt.Sprintf("%s=%s", name, lock.Version))
			p.LockedPackages = append(p.LockedPackages, string(name))
			p.DebHashes = append(p.DebHashes, fmt.Sprintf("%s\t%s", lock.DebHash, lock.DebFilename))
		}
	}
	sort.Strings(p.LockedPackageSpecs)
	sort.Strings(p.LockedPackages)
	sort.Strings(p.DebHashes)

	if err := dockerfileTemplate.Execute(&buf, p); err != nil {
		return "", fmt.Errorf("rendering dockerfile template: %w", err)
	}

	// Trim empty lines, so the template can be less awkward about newlines:
	var ret strings.Builder
	for _, line := range strings.Split(buf.String(), "\n") {
		l := strings.TrimSpace(line)
		if l != "" {
			ret.WriteString(line)
			ret.WriteRune('\n')
		}
	}
	return ret.String(), nil
}

func baseImage(mf manifest.Manifest) string {
	if mf.DpkgLockJSON != nil {
		return mf.DpkgLockJSON.Image
	}
	return fmt.Sprintf("debian:%s-slim", mf.DpkgJSON.Distro)
}
