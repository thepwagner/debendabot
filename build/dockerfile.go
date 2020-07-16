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

{{/* 
	TODO: maybe debootstrap?
	expensive, but lets us provide as hash of all the .debs. that would be cool.
*/}}

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

{{ if .LockedPackages }}
RUN apt-get install -y \
{{ range $packageSpec := .LockedPackageSpecs }}
	{{$packageSpec}} \
{{ end }}
  && apt-mark auto \
{{ range $packageSpec := .LockedPackages }}
	{{$packageSpec}} \
{{ end }}
  && true
{{ end }}
RUN apt-get install -y --no-install-recommends \
{{ range $packageSpec := .PackageSpecs }}
	{{$packageSpec}} \
{{ end }}
  && true

FROM build AS manifest
RUN apt list --installed -qq | tee /apt-installed.txt

FROM build
{{if .Proxy}}
ENV http_proxy=
{{end}}
`))

type dockerfileTemplateParams struct {
	BaseImage          string
	LockedPackageSpecs []string
	LockedPackages     []string
	PackageSpecs       []string
	Proxy              string
}

func genDockerfile(mf manifest.Manifest) (string, error) {
	var buf strings.Builder

	p := dockerfileTemplateParams{
		// FIXME: don't hardcode my apt-cacher
		Proxy: "http://172.17.0.1:3142",
	}
	p.BaseImage = baseImage(mf)

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
		}
	}
	sort.Strings(p.LockedPackageSpecs)
	sort.Strings(p.LockedPackages)

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
