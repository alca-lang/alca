package apm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// PublishOptions configure a publish operation.
type PublishOptions struct {
	// Dir is the library directory containing alca.toml.
	Dir string
	// RegistryURL is the registry node to publish to.
	RegistryURL string
	// Token authenticates the author (Bearer token).
	Token string
	// InsecureTLS disables certificate verification (development only).
	InsecureTLS bool
	// AllowInsecureHTTP permits plain-http registries (loopback is always allowed).
	AllowInsecureHTTP bool
	// Targets are "goos/goarch" pairs to compile and upload. Default: host only.
	Targets []string
	// Out receives progress messages; nil discards them.
	Out io.Writer
	// HTTPClient is used internally; tests may inject one.
	HTTPClient HTTPDoer
}

func (o *PublishOptions) logf(format string, args ...any) {
	if o.Out == nil {
		return
	}
	fmt.Fprintf(o.Out, format+"\n", args...)
}

// PublishResult reports a successful publish.
type PublishResult struct {
	Name          string
	Version       string
	SHA256        string
	AlreadyStored bool
	Artifacts     []ArtifactInfo
}

// Publish validates a library, packs it, compiles builds for the requested
// targets and uploads everything to a registry node. It refuses to overwrite a
// published version and verifies the connection is secure.
func Publish(opts PublishOptions) (*PublishResult, error) {
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.Token) == "" {
		return nil, fmt.Errorf("publishing requires an author token; set %s or pass --token", EnvToken)
	}
	if err := CheckSecureRegistryURL(opts.RegistryURL, opts.AllowInsecureHTTP); err != nil {
		return nil, err
	}

	opts.logf("validating %s", abs)
	m, err := ValidatePackageDir(abs)
	if err != nil {
		return nil, err
	}
	if !m.Build.Compile {
		opts.logf("note: build.compile = false; publishing sources only")
	}
	archive, err := PackDir(abs)
	if err != nil {
		return nil, err
	}
	sum := sha256Hex(archive)

	client := NewClient(opts.RegistryURL, ClientOptions{
		Token:       opts.Token,
		InsecureTLS: opts.InsecureTLS,
		HTTPClient:  opts.HTTPClient,
	})

	// Pre-flight: never overwrite a published version. Identical content means
	// a previous attempt already succeeded.
	if existing, err := client.Version(m.Package.Name, m.Package.Version); err == nil && existing != nil {
		if existing.SHA256 == sum {
			opts.logf("%s %s is already published (identical archive)", m.Package.Name, m.Package.Version)
			return &PublishResult{Name: m.Package.Name, Version: m.Package.Version, SHA256: sum, AlreadyStored: true, Artifacts: existing.Artifacts}, nil
		}
		return nil, fmt.Errorf("%s %s is already published with different content; bump the version in alca.toml", m.Package.Name, m.Package.Version)
	}

	targets, err := publishTargets(opts.Targets)
	if err != nil {
		return nil, err
	}

	var artifacts []ArtifactInfo
	for _, target := range targets {
		if !m.Build.Compile {
			break
		}
		meta, err := BuildArtifact(abs, m, target.goos, target.goarch)
		if err != nil {
			if isUnsupported(err) {
				opts.logf("note: %s/%s build skipped: %v", target.goos, target.goarch, err)
				continue
			}
			return nil, err
		}
		binRel, _ := ArtifactPaths(meta.GOOS, meta.GOARCH)
		opts.logf("compiled %s/%s build (%d bytes)", meta.GOOS, meta.GOARCH, meta.Size)
		artifacts = append(artifacts, ArtifactInfo{
			GOOS:           meta.GOOS,
			GOARCH:         meta.GOARCH,
			AlcaVersion:    meta.AlcaVersion,
			PayloadVersion: meta.PayloadVersion,
			SHA256:         meta.SHA256,
			Size:           meta.Size,
			Entry:          meta.Entry,
			SourceHash:     meta.SourceHash,
			Exports:        meta.Exports,
			CreatedAt:      meta.CreatedAt,
		})
		_ = binRel
	}

	opts.logf("uploading %s %s (%d bytes)", m.Package.Name, m.Package.Version, len(archive))
	resp, err := client.Publish(archive)
	if err != nil {
		return nil, fmt.Errorf("publish failed: %w", err)
	}
	result := &PublishResult{Name: m.Package.Name, Version: m.Package.Version, SHA256: sum, Artifacts: artifacts}
	if resp.Status == "already-published" {
		result.AlreadyStored = true
		return result, nil
	}

	for _, info := range artifacts {
		binRel, metaRel := ArtifactPaths(info.GOOS, info.GOARCH)
		bin, err := os.ReadFile(filepath.Join(abs, filepath.FromSlash(binRel)))
		if err != nil {
			return nil, fmt.Errorf("cannot read compiled build: %w", err)
		}
		metaData, err := os.ReadFile(filepath.Join(abs, filepath.FromSlash(metaRel)))
		if err != nil {
			return nil, fmt.Errorf("cannot read build metadata: %w", err)
		}
		var meta ArtifactMeta
		if err := json.Unmarshal(metaData, &meta); err != nil {
			return nil, fmt.Errorf("cannot parse build metadata: %w", err)
		}
		opts.logf("uploading %s/%s build", info.GOOS, info.GOARCH)
		if err := client.UploadArtifact(m.Package.Name, m.Package.Version, info.GOOS, info.GOARCH, meta, bin); err != nil {
			return nil, fmt.Errorf("uploading %s/%s build: %w", info.GOOS, info.GOARCH, err)
		}
	}
	return result, nil
}

type publishTarget struct{ goos, goarch string }

func publishTargets(list []string) ([]publishTarget, error) {
	if len(list) == 0 {
		goos, goarch := HostTarget()
		return []publishTarget{{goos, goarch}}, nil
	}
	seen := map[string]bool{}
	var out []publishTarget
	for _, t := range list {
		parts := strings.Split(strings.TrimSpace(t), "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid target %q (expected os/arch, e.g. linux/amd64)", t)
		}
		key := parts[0] + "/" + parts[1]
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, publishTarget{parts[0], parts[1]})
	}
	return out, nil
}

// CheckSecureRegistryURL refuses plain http registries unless they are on the
// loopback interface or insecure mode was explicitly requested. Tokens and
// packages must never travel unencrypted to a public host.
func CheckSecureRegistryURL(raw string, allowInsecure bool) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("no registry URL configured; pass --registry, set %s, or use a build with a built-in registry node", EnvRegistry)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid registry URL %q", raw)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if allowInsecure || host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}
		return fmt.Errorf("refusing to send packages and tokens over plain http to %s; use https or pass --allow-insecure-http", host)
	default:
		return fmt.Errorf("unsupported registry URL scheme %q (use https://)", u.Scheme)
	}
}
