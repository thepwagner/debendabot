package build

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/thepwagner/debendabot/manifest"
)

var dockerfileTemplate = template.Must(template.New("dockerfile").Parse(`
FROM debian:{{.Distro}}-slim AS base

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
	*manifest.DpkgJSON
	PackageSpecs []string
	Proxy        string
}

func genDockerfile(dpkgJSON *manifest.DpkgJSON) (string, error) {
	var buf strings.Builder

	p := dockerfileTemplateParams{
		DpkgJSON: dpkgJSON,
		// FIXME: don't hardcode my apt-cacher
		Proxy: "http://172.17.0.1:3142",
	}
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
	return buf.String(), nil
}
