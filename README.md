# Deb-endabot

## This is not endorsed by or associated with GitHub, Dependabot, etc.

It's not even endorsed by me, strictly for science and lulz.

## What is this?

This is an experiment for building verifiable Debian root filesystems (e.g. for Docker images), and indexing such filesystems' contents.

The idea is to provide a system like NPM's `package.json` and `package-lock.json` that operate on .deb archives:

* [gnupg](https://github.com/thepwagner/debendabot/tree/main/examples/gnupg)
* [zsh](https://github.com/thepwagner/debendabot/tree/main/examples/zsh)

Those files drive a [Dockerfile](https://github.com/thepwagner/debendabot/blob/main/build/dockerfile.go) template that assembles the image.
