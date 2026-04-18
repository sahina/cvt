# Reference plugins (staging)

This directory holds the source for the two first-party CVT plugins
while they incubate alongside the plugin framework. On first tagged
release, each subdirectory moves to its own repo:

- `cvt-plugin-registry-rest/` → `github.com/sahina/cvt-plugin-registry-rest`
- `cvt-plugin-slack-events/` → `github.com/sahina/cvt-plugin-slack-events`

Extraction is a mechanical `git filter-repo` on the subdirectory.

While staged here, each plugin has its own `go.mod` and is independent
of CVT core's module. This ensures extraction is lossless and forces
the plugins to consume `github.com/sahina/cvt/pkg/cvtplugin` through
its public surface the same way external plugins will.
