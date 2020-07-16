package build

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/thepwagner/debendabot/manifest"
)

var dockerfileTemplate = template.Must(template.New("dockerfile").Parse(`
FROM {{.BaseImage}} AS base

{{/* 
	TODO: maybe debootstrap?
	Expensive, but lets us provide as hash of all the .debs. that would be cool.
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
RUN apt-get install -y \{{ range $packageSpec := .PackageSpecs }}
	{{$packageSpec}} \{{ end }}
  && true  {{/* avoid empty continuation */}}

FROM build AS manifest
RUN apt list --installed -qq | tee /apt-installed.txt

FROM build
{{if .Proxy}}
ENV http_proxy=
{{end}}
`))

type dockerfileTemplateParams struct {
	manifest.DpkgJSON
	BaseImage    string
	PackageSpecs []string
	Proxy        string
}

func genDockerfile(mf manifest.Manifest) (string, error) {
	var buf strings.Builder

	p := dockerfileTemplateParams{
		DpkgJSON: mf.DpkgJSON,
		// FIXME: don't hardcode my apt-cacher
		Proxy: "http://172.17.0.1:3142",
	}
	p.BaseImage = baseImage(mf)

	for name, version := range p.Packages {
		switch version {
		case "stable", "unstable", "testing":
			p.PackageSpecs = append(p.PackageSpecs, fmt.Sprintf("%s/%s", name, version))
		default:
			// XXX: this requires an exact match - could we filter candidates against semver?
			// not all packages _are_ semver; some library can handle that? :crossed_fingers:
			p.PackageSpecs = append(p.PackageSpecs, fmt.Sprintf("%s=%s", name, version))
		}
	}

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
