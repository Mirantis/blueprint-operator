# The default naming convention for the operator is for a development image
# on a developer's machine. This is so that devs don't have to remember to set
# these values. CI will set vars so that the image is tagged correctly during
# a CI build.
COMMIT=$(shell git rev-parse --short HEAD)
DATE=$(shell date -u '+%Y-%m-%d')
IMAGE_REPO?=ghcr.io/mirantiscontainers
IMAGE_NAME?=mke-operator

# A basic dev tag is by default so that the same image is rebuilt during development
VERSION?=dev
ifdef RELEASE_BUILD
    VERSION=$(shell git tag -l "v*.*.*" --sort=-version:refname | head -n 1)
endif
ifdef MERGE_BUILD
	# This will replace the last 2 '-' characters with '+' to make it a valid semver with build info
	# Assumes that the version is in the format vX.Y.Z-<commit count>-<sha>
    VERSION=$(shell git describe --tags --always | sed 's/\(.*\)-/\1+/' | sed 's/\(.*\)-/\1+/')
endif

# Setup the full image name
IMAGE_TAG_BASE=$(IMAGE_REPO)/$(IMAGE_NAME)
IMG:=$(IMAGE_TAG_BASE):$(VERSION)

# Docker doesn't allow '+' in the image tag, so merge builds only use the commit
# sha in the image tag.
ifdef MERGE_BUILD
	IMG:=$(IMAGE_TAG_BASE):$(COMMIT)
endif
