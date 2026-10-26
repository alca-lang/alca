// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package apm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/alca-lang/alca/pkg/compiler"
	"github.com/alca-lang/alca/pkg/version"
)

// InstallOptions configure installation behaviour.
type InstallOptions struct {
	// RegistryURL is the remote registry node to fetch packages from. Empty
	// means offline: only already-installed dependencies may be used.
	RegistryURL string
	// InsecureTLS disables TLS certificate verification (for development).
	InsecureTLS bool
	// Force allows replacing versions that installed packages depend on.
	Force bool
	// Out receives progress messages; nil discards them.
	Out io.Writer
	// HTTPClient is used internally; tests may inject one.
	HTTPClient HTTPDoer
}

// InstalledPackage describes one installed package version.
type InstalledPackage struct {
	Name          string
	Version       string
	Description   string
	Entry         string
	Alca          string
	License       string
	Path          string
	Origin        string
	Dependencies  map[string]string
	Artifacts     []ArtifactEntry
	InstalledAt   string
	AlreadyExists bool // set by Install when the version was already present
}

// Installer performs package operations against a local registry.
type Installer struct {
	reg  *Registry
	opts InstallOptions
}

// NewInstaller creates an installer for a local registry.
func NewInstaller(reg *Registry, opts InstallOptions) *Installer {
	return &Installer{reg: reg, opts: opts}
}

func (in *Installer) logf(format string, args ...any) {
	if in.opts.Out == nil {
		return
	}
	fmt.Fprintf(in.opts.Out, format+"\n", args...)
}

func (in *Installer) client() *Client {
	if strings.TrimSpace(in.opts.RegistryURL) == "" {
		return nil
	}
	return NewClient(in.opts.RegistryURL, ClientOptions{
		InsecureTLS: in.opts.InsecureTLS,
		HTTPClient:  in.opts.HTTPClient,
	})
}

// currentVersion returns the running toolchain version.
func currentVersion() string { return version.Version }

// HostTarget returns the goos/goarch of the running process.
func HostTarget() (string, string) { return runtime.GOOS, runtime.GOARCH }

// ---------------------------------------------------------------------------
// Spec parsing
// ---------------------------------------------------------------------------

// ParseSpec parses "name", "name@1.2.3", "name@^1.2" or "name@>=1.0.0 <2.0.0".
func ParseSpec(spec string) (string, Constraint, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", Constraint{}, fmt.Errorf("empty package name")
	}
	name := spec
	constraintText := ""
	if i := strings.IndexByte(spec, '@'); i >= 0 {
		name, constraintText = spec[:i], spec[i+1:]
	}
	if !ValidName(name) {
		return "", Constraint{}, fmt.Errorf("invalid package name %q", name)
	}
	c, err := ParseConstraint(constraintText)
	if err != nil {
		return "", Constraint{}, fmt.Errorf("invalid version for %s: %w", name, err)
	}
	return name, c, nil
}

type requirement struct {
	name       string
	constraint Constraint
	requiredBy string
}

// ---------------------------------------------------------------------------
// Dependency resolution
// ---------------------------------------------------------------------------

// resolveRemote computes the full dependency plan for the requested roots: one
// version per package that satisfies every constraint, with dependencies first.
func (in *Installer) resolveRemote(roots []requirement) ([]VersionInfo, error) {
	client := in.client()
	if client == nil {
		return nil, fmt.Errorf("no registry configured; pass --registry, set %s, or use a build with a built-in registry node", EnvRegistry)
	}
	cache := map[string][]VersionInfo{}
	getVersions := func(name string) ([]VersionInfo, error) {
		if vs, ok := cache[name]; ok {
			return vs, nil
		}
		vs, err := client.Versions(name)
		if err != nil {
			if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
				return nil, fmt.Errorf("\033[31mPackage '%s' not found in registry\033[0m", name)
			}
			return nil, cleanRegistryError(err)
		}
		if len(vs) == 0 {
			return nil, fmt.Errorf("\033[31mPackage '%s' not found in registry\033[0m", name)
		}
		cache[name] = vs
		return vs, nil
	}

	selected := map[string]VersionInfo{}
	for iter := 0; iter < 64; iter++ {
		constraints := map[string][]requirement{}
		for _, r := range roots {
			constraints[r.name] = append(constraints[r.name], r)
		}
		for name, info := range selected {
			for dep, spec := range info.Dependencies {
				c, err := ParseConstraint(spec)
				if err != nil {
					return nil, fmt.Errorf("package %s@%s declares an invalid dependency %s = %q", name, info.Version, dep, spec)
				}
				constraints[dep] = append(constraints[dep], requirement{dep, c, name + "@" + info.Version})
			}
		}
		next := map[string]VersionInfo{}
		for _, name := range sortedKeys(constraints) {
			reqs := constraints[name]
			vs, err := getVersions(name)
			if err != nil {
				return nil, err
			}
			best := pickBestVersion(vs, reqs)
			if best == nil {
				return nil, constraintConflict(name, reqs, vs)
			}
			next[name] = *best
		}
		if versionsEqual(selected, next) {
			selected = next
			break
		}
		selected = next
		if iter == 63 {
			return nil, fmt.Errorf("dependency resolution did not converge; check for cyclic version constraints")
		}
	}
	return topoOrder(selected)
}

func versionsEqual(a, b map[string]VersionInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for name, va := range a {
		vb, ok := b[name]
		if !ok || va.Version != vb.Version || va.SHA256 != vb.SHA256 {
			return false
		}
	}
	return true
}

func pickBestVersion(versions []VersionInfo, reqs []requirement) *VersionInfo {
	var matches []VersionInfo
	for _, v := range versions {
		if v.Yanked {
			continue
		}
		parsed, err := ParseVersion(v.Version)
		if err != nil {
			continue
		}
		ok := true
		for _, r := range reqs {
			if !r.constraint.Satisfies(parsed) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if compat, err := AlcaCompatible(v.Alca); err != nil || !compat {
			continue
		}
		matches = append(matches, v)
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		vi, _ := ParseVersion(matches[i].Version)
		vj, _ := ParseVersion(matches[j].Version)
		return vj.Less(vi)
	})
	// Prefer the newest stable release unless nothing else matches.
	prereleaseWanted := false
	for _, r := range reqs {
		for _, a := range r.constraint.Atoms {
			if a.Version.Pre != "" {
				prereleaseWanted = true
			}
		}
	}
	if !prereleaseWanted {
		for i := range matches {
			v, _ := ParseVersion(matches[i].Version)
			if v.Pre == "" {
				return &matches[i]
			}
		}
	}
	return &matches[0]
}

func constraintConflict(name string, reqs []requirement, versions []VersionInfo) error {
	var lines []string
	seen := map[string]bool{}
	for _, r := range reqs {
		key := r.requiredBy + "|" + r.constraint.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		who := r.requiredBy
		if who == "" {
			who = "requested"
		}
		c := r.constraint.String()
		if c == "" {
			c = "*"
		}
		lines = append(lines, fmt.Sprintf("  %s requires %s %s", who, name, c))
	}
	var avail []string
	for _, v := range versions {
		avail = append(avail, v.Version)
	}
	sort.Strings(avail)
	return fmt.Errorf("no version of %s satisfies all constraints:\n%s\navailable versions: %s",
		name, strings.Join(lines, "\n"), strings.Join(avail, ", "))
}

// topoOrder returns the selected packages with dependencies before dependents.
func topoOrder(selected map[string]VersionInfo) ([]VersionInfo, error) {
	state := map[string]int{}
	var order []VersionInfo
	var visit func(name string, chain []string) error
	visit = func(name string, chain []string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("dependency cycle: %s -> %s", strings.Join(chain, " -> "), name)
		case 2:
			return nil
		}
		state[name] = 1
		info := selected[name]
		for _, dep := range sortedKeys(info.Dependencies) {
			if _, ok := selected[dep]; ok {
				if err := visit(dep, append(chain, name)); err != nil {
					return err
				}
			}
		}
		state[name] = 2
		order = append(order, info)
		return nil
	}
	for _, name := range sortedKeys(selected) {
		if err := visit(name, nil); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// ---------------------------------------------------------------------------
// Install / remove / show
// ---------------------------------------------------------------------------

// Install installs packages (with their dependencies) from the remote registry.
func (in *Installer) Install(specs ...string) ([]InstalledPackage, error) {
	idx, err := in.reg.LoadIndex()
	if err != nil {
		return nil, err
	}
	var roots []requirement
	for _, spec := range specs {
		name, constraint, err := ParseSpec(spec)
		if err != nil {
			return nil, err
		}
		if installed, ok, err := idx.Installed(name); err != nil {
			return nil, err
		} else if ok {
			if v, err := ParseVersion(installed.Version); err == nil && constraint.Satisfies(v) {
				in.logf("  %s %s is already installed", name, installed.Version)
				continue
			}
		}
		roots = append(roots, requirement{name: name, constraint: constraint, requiredBy: "requested"})
	}
	if len(roots) == 0 {
		return nil, nil
	}

	client := in.client()
	if client != nil {
		for _, r := range roots {
			exists, vs, err := client.PackageExists(r.name)
			if err != nil {
				// Registry unreachable or timed out (15s): fallback to local cache
				in.logf("  registry unreachable; falling back to local cache")
				return in.installFromCache(idx, roots)
			}
			if !exists || len(vs) == 0 {
				redMsg := fmt.Sprintf("\033[31mPackage '%s' not found in registry\033[0m", r.name)
				return nil, errors.New(redMsg)
			}
		}
	} else {
		return in.installFromCache(idx, roots)
	}

	plan, err := in.resolveRemote(roots)
	if err != nil {
		return nil, err
	}
	return in.applyPlan(idx, plan)
}

func (in *Installer) installFromCache(idx *Index, roots []requirement) ([]InstalledPackage, error) {
	var results []InstalledPackage
	for _, r := range roots {
		entries, err := os.ReadDir(in.reg.CacheDir())
		if err != nil {
			return nil, fmt.Errorf("fallback: library '%s' not found in local cache", r.name)
		}
		var matchedVer string
		var matchedArchive []byte
		prefix := r.name + "-"
		suffix := ".tar.gz"
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), suffix) {
				verStr := strings.TrimSuffix(strings.TrimPrefix(e.Name(), prefix), suffix)
				if v, err := ParseVersion(verStr); err == nil && r.constraint.Satisfies(v) {
					data, err := os.ReadFile(filepath.Join(in.reg.CacheDir(), e.Name()))
					if err == nil {
						matchedVer = verStr
						matchedArchive = data
						break
					}
				}
			}
		}
		if matchedArchive == nil {
			return nil, fmt.Errorf("fallback: library '%s' not found in local cache", r.name)
		}
		info := VersionInfo{Name: r.name, Version: matchedVer}
		installed, err := in.installArchive(idx, info, matchedArchive, "cache", nil)
		if err != nil {
			return nil, err
		}
		in.logf("  installed %s %s (from cache)", installed.Name, installed.Version)
		results = append(results, *installed)
	}
	return results, nil
}

func formatSize(b int64) string {
	if b <= 0 {
		return "0 B"
	}
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	kb := float64(b) / 1024.0
	if kb < 1024.0 {
		return fmt.Sprintf("%.1f KB", kb)
	}
	mb := kb / 1024.0
	return fmt.Sprintf("%.1f MB", mb)
}

func formatSpeed(bps float64) string {
	if bps <= 0 {
		return "0 B/s"
	}
	if bps < 1024.0 {
		return fmt.Sprintf("%.0f B/s", bps)
	}
	kbps := bps / 1024.0
	if kbps < 1024.0 {
		return fmt.Sprintf("%.1f KB/s", kbps)
	}
	mbps := kbps / 1024.0
	return fmt.Sprintf("%.1f MB/s", mbps)
}

func renderProgressBar(name, ver string, downloaded, total int64, speed float64, done bool) string {
	const barWidth = 15
	var ratio float64
	if total > 0 {
		ratio = float64(downloaded) / float64(total)
		if ratio > 1.0 {
			ratio = 1.0
		}
	} else if done {
		ratio = 1.0
	}
	filled := int(ratio * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("=", filled) + strings.Repeat(" ", barWidth-filled)
	pct := int(ratio * 100)
	sizeStr := formatSize(downloaded)
	if total > 0 && !done {
		sizeStr = fmt.Sprintf("%s/%s", formatSize(downloaded), formatSize(total))
	}
	speedStr := formatSpeed(speed)
	if speed <= 0 && done {
		return fmt.Sprintf("  %s %s [%s] %3d%% (%s)", name, ver, bar, pct, sizeStr)
	}
	return fmt.Sprintf("  %s %s [%s] %3d%% (%s, %s)", name, ver, bar, pct, speedStr, sizeStr)
}

func (in *Installer) downloadWithProgress(client *Client, info VersionInfo) ([]byte, error) {
	if client == nil {
		data, ok := in.reg.CachedArchive(info.Name, info.Version)
		if !ok {
			return nil, fmt.Errorf("package %s %s not found in local cache", info.Name, info.Version)
		}
		if in.opts.Out != nil {
			fmt.Fprintln(in.opts.Out, renderProgressBar(info.Name, info.Version, int64(len(data)), int64(len(data)), 0, true))
		}
		return data, nil
	}

	var lastLineLen int
	onProgress := func(downloaded, total int64, speed float64) {
		if in.opts.Out == nil {
			return
		}
		isDone := total > 0 && downloaded >= total
		bar := renderProgressBar(info.Name, info.Version, downloaded, total, speed, isDone)
		pad := ""
		if len(bar) < lastLineLen {
			pad = strings.Repeat(" ", lastLineLen-len(bar))
		}
		lastLineLen = len(bar)
		if isDone {
			fmt.Fprintf(in.opts.Out, "\r%s%s\n", bar, pad)
		} else {
			fmt.Fprintf(in.opts.Out, "\r%s%s", bar, pad)
		}
	}

	archive, err := client.DownloadWithProgress(info.Name, info.Version, info.SHA256, onProgress)
	if err != nil {
		if in.opts.Out != nil {
			fmt.Fprintln(in.opts.Out)
		}
		return nil, cleanRegistryError(err)
	}
	if in.opts.Out != nil && lastLineLen > 0 {
		finalBar := renderProgressBar(info.Name, info.Version, int64(len(archive)), int64(len(archive)), 0, true)
		fmt.Fprintf(in.opts.Out, "\r%s\n", finalBar)
	}
	return archive, nil
}

func (in *Installer) applyPlan(idx *Index, plan []VersionInfo) ([]InstalledPackage, error) {
	if !in.opts.Force {
		if err := in.checkInstalledDependents(idx, plan); err != nil {
			return nil, err
		}
	}
	client := in.client()
	var results []InstalledPackage
	for _, info := range plan {
		if installed, ok, err := idx.Installed(info.Name); err != nil {
			return nil, err
		} else if ok && installed.Version == info.Version {
			in.logf("  %s %s is already installed", info.Name, info.Version)
			results = append(results, entryToPackage(installed))
			results[len(results)-1].AlreadyExists = true
			continue
		}

		archive, err := in.downloadWithProgress(client, info)
		if err != nil {
			return nil, err
		}
		if err := in.reg.CacheArchive(info.Name, info.Version, archive); err != nil {
			in.logf("  warning: cannot cache %s: %v", info.Name, err)
		}

		installed, err := in.installArchive(idx, info, archive, "remote:"+in.opts.RegistryURL, in.fetchRemoteBuild(client, info))
		if err != nil {
			return nil, err
		}
		in.logf("  installed %s %s", installed.Name, installed.Version)
		results = append(results, *installed)
	}
	return results, nil
}

// checkInstalledDependents verifies that upgrading packages in plan does not
// break the version constraints of everything else that is installed.
func (in *Installer) checkInstalledDependents(idx *Index, plan []VersionInfo) error {
	planned := map[string]string{}
	for _, info := range plan {
		planned[info.Name] = info.Version
	}
	var problems []string
	for name, entry := range idx.Packages {
		for _, ve := range entry.Versions {
			if _, upgraded := planned[name]; upgraded {
				continue
			}
			for dep, spec := range ve.Dependencies {
				target, ok := planned[dep]
				if !ok {
					if inst, ok, _ := idx.Installed(dep); ok {
						target = inst.Version
					} else {
						problems = append(problems, fmt.Sprintf("%s %s needs %s %s, which is not installed", name, ve.Version, dep, spec))
						continue
					}
				}
				c, err := ParseConstraint(spec)
				if err != nil {
					continue
				}
				v, err := ParseVersion(target)
				if err != nil || !c.Satisfies(v) {
					problems = append(problems, fmt.Sprintf("%s %s needs %s %s, but %s would be installed", name, ve.Version, dep, spec, target))
				}
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("this install would break installed packages:\n  %s\nuse --force to override", strings.Join(problems, "\n  "))
	}
	return nil
}

// installArchive validates an archive, stages it, compiles its artifact and
// atomically moves it into the registry. rb, when non-nil, is a compiled build
// downloaded from the registry node that replaces local compilation.
func (in *Installer) installArchive(idx *Index, info VersionInfo, archive []byte, origin string, rb *remoteBuild) (*InstalledPackage, error) {
	staging, err := os.MkdirTemp(in.reg.TempDir(), "stage-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)

	if err := UnpackArchive(archive, staging); err != nil {
		return nil, err
	}
	m, err := ReadManifest(staging)
	if err != nil {
		return nil, err
	}
	if m.Package.Name != info.Name {
		return nil, fmt.Errorf("package archive declares name %q but %q was requested", m.Package.Name, info.Name)
	}
	if m.Package.Version != info.Version {
		return nil, fmt.Errorf("package archive declares version %q but %q was requested", m.Package.Version, info.Version)
	}
	if err := parseAllSources(staging); err != nil {
		return nil, err
	}
	if entry := filepath.Join(staging, filepath.FromSlash(cleanRel(m.Package.Entry))); !fileExists(entry) {
		return nil, fmt.Errorf("package %s %s is missing its entry %s", info.Name, info.Version, m.Package.Entry)
	}

	// A downloaded build is only trusted when it was compiled from exactly
	// these sources; otherwise fall back to compiling locally.
	if rb != nil {
		if rb.info.Entry != cleanRel(m.Package.Entry) {
			in.logf("  note: published build for %s %s targets entry %s; compiling locally",
				info.Name, info.Version, rb.info.Entry)
			rb = nil
		} else if srcHash, err := SourceHash(staging); err != nil {
			in.logf("  note: cannot hash %s sources (%v); compiling locally", info.Name, err)
			rb = nil
		} else if rb.info.SourceHash == "" || rb.info.SourceHash != srcHash {
			in.logf("  note: published build for %s %s does not match its sources; compiling locally",
				info.Name, info.Version)
			rb = nil
		}
	}

	return in.finalizeInstall(idx, staging, m, origin, rb)
}

// remoteBuild is a compiled build fetched from a registry node.
type remoteBuild struct {
	bin  []byte
	info ArtifactInfo
}

// fetchRemoteBuild returns the node's compiled build for this host when one was
// published and looks compatible. Every mismatch simply means "compile
// locally": sources are always available.
func (in *Installer) fetchRemoteBuild(client *Client, info VersionInfo) *remoteBuild {
	goos, goarch := HostTarget()
	var picked *ArtifactInfo
	for i := range info.Artifacts {
		a := &info.Artifacts[i]
		if a.GOOS == goos && a.GOARCH == goarch {
			picked = a
			break
		}
	}
	if picked == nil {
		return nil
	}
	if picked.AlcaVersion != "" && picked.AlcaVersion != currentVersion() {
		in.logf("  note: node build for %s %s was made by alca %s; compiling locally with %s",
			info.Name, info.Version, picked.AlcaVersion, currentVersion())
		return nil
	}
	if picked.PayloadVersion != compiler.CurrentPayloadVersion {
		in.logf("  note: node build for %s %s uses payload %d (this runtime speaks %d); compiling locally",
			info.Name, info.Version, picked.PayloadVersion, compiler.CurrentPayloadVersion)
		return nil
	}
	bin, err := client.DownloadArtifact(info.Name, info.Version, goos, goarch, picked.SHA256)
	if err != nil {
		in.logf("  warning: cannot fetch the compiled build for %s %s (%v); compiling locally",
			info.Name, info.Version, err)
		return nil
	}
	in.logf("  using compiled build %s %s (%s/%s, sha256:%s)", info.Name, info.Version, goos, goarch, shortSHA(picked.SHA256))
	return &remoteBuild{bin: bin, info: *picked}
}

// artifactMetaFromInfo rebuilds the artifact sidecar metadata of a build that
// arrived over the wire.
func artifactMetaFromInfo(name, version, srcHash string, a ArtifactInfo) *ArtifactMeta {
	return &ArtifactMeta{
		Format:         ArtifactFormat,
		FormatVersion:  ArtifactFormatVersion,
		Name:           name,
		Version:        version,
		Entry:          a.Entry,
		GOOS:           a.GOOS,
		GOARCH:         a.GOARCH,
		AlcaVersion:    a.AlcaVersion,
		PayloadVersion: a.PayloadVersion,
		SourceHash:     srcHash,
		Exports:        a.Exports,
		CreatedAt:      a.CreatedAt,
		SHA256:         a.SHA256,
		Size:           a.Size,
	}
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// finalizeInstall builds (or installs a downloaded) artifact, moves the staged
// directory into place and updates the index. staging is consumed.
func (in *Installer) finalizeInstall(idx *Index, staging string, m *Manifest, origin string, rb *remoteBuild) (*InstalledPackage, error) {
	goos, goarch := HostTarget()
	artifacts := map[string]*ArtifactEntry{}
	if m.Build.Compile {
		switch {
		case rb != nil:
			srcHash, err := SourceHash(staging)
			if err != nil {
				return nil, err
			}
			meta := artifactMetaFromInfo(m.Package.Name, m.Package.Version, srcHash, rb.info)
			binRel, metaRel := ArtifactPaths(meta.GOOS, meta.GOARCH)
			if err := writeArtifactFiles(staging, binRel, metaRel, rb.bin, meta); err != nil {
				return nil, err
			}
			artifacts[meta.GOOS+"/"+meta.GOARCH] = &ArtifactEntry{
				File:           binRel,
				MetaFile:       metaRel,
				SHA256:         meta.SHA256,
				AlcaVersion:    meta.AlcaVersion,
				PayloadVersion: meta.PayloadVersion,
				GOOS:           meta.GOOS,
				GOARCH:         meta.GOARCH,
			}
		default:
			meta, err := BuildArtifact(staging, m, goos, goarch)
			if err != nil {
				if !isUnsupported(err) {
					return nil, err
				}
				in.logf("  note: %s %s has no compiled build (%v); sources will be used", m.Package.Name, m.Package.Version, err)
			} else {
				key := goos + "/" + goarch
				artifacts[key] = &ArtifactEntry{
					File:           metaEntryFile(meta, true),
					MetaFile:       metaEntryFile(meta, false),
					SHA256:         meta.SHA256,
					AlcaVersion:    meta.AlcaVersion,
					PayloadVersion: meta.PayloadVersion,
					GOOS:           goos,
					GOARCH:         goarch,
				}
			}
		}
	}

	target := in.reg.PackageDir(m.Package.Name, m.Package.Version)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}
	if dirExists(target) {
		if err := os.RemoveAll(target); err != nil {
			return nil, fmt.Errorf("cannot replace %s: %w", target, err)
		}
	}
	if err := os.Rename(staging, target); err != nil {
		return nil, fmt.Errorf("cannot move package into place: %w", err)
	}

	srcHash, _ := SourceHash(target)
	relPath, err := filepath.Rel(in.reg.Dir(), target)
	if err != nil {
		relPath = target
	}
	ve := &VersionEntry{
		Name:         m.Package.Name,
		Version:      m.Package.Version,
		Description:  m.Package.Description,
		Entry:        cleanRel(m.Package.Entry),
		Alca:         m.Package.Alca,
		License:      m.Package.License,
		Dependencies: m.Dependencies,
		Path:         filepath.ToSlash(relPath),
		SourceHash:   srcHash,
		InstalledAt:  time.Now().UTC().Format(time.RFC3339),
		Origin:       origin,
		Artifacts:    artifacts,
	}

	// Flat local registry: only the newest version of a package stays usable.
	if entry, ok := idx.Packages[m.Package.Name]; ok {
		for v := range entry.Versions {
			if v == m.Package.Version {
				continue
			}
			if err := os.RemoveAll(in.reg.PackageDir(m.Package.Name, v)); err == nil || os.IsNotExist(err) {
				delete(entry.Versions, v)
			}
		}
	}
	idx.SetVersion(m.Package.Name, ve)
	if err := in.reg.SaveIndex(idx); err != nil {
		return nil, err
	}
	pkg := entryToPackage(ve)
	pkg.Path = target
	return &pkg, nil
}

// metaEntryFile returns the artifact file path recorded in the index.
func metaEntryFile(meta *ArtifactMeta, bin bool) string {
	b, m := ArtifactPaths(meta.GOOS, meta.GOARCH)
	if bin {
		return b
	}
	return m
}

// writeArtifactFiles stores a downloaded build and its metadata sidecar inside
// a package directory.
func writeArtifactFiles(dir, binRel, metaRel string, bin []byte, meta *ArtifactMeta) error {
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicWriteFile(filepath.Join(dir, filepath.FromSlash(binRel)), bin, 0o644); err != nil {
		return err
	}
	return atomicWriteFile(filepath.Join(dir, filepath.FromSlash(metaRel)), metaData, 0o644)
}

func isUnsupported(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrArtifactUnsupported.Error())
}

// InstallLocal installs a library from a local source directory.
func (in *Installer) InstallLocal(dir string) (*InstalledPackage, error) {
	m, err := CheckPackageStructure(dir)
	if err != nil {
		return nil, err
	}
	idx, err := in.reg.LoadIndex()
	if err != nil {
		return nil, err
	}
	// Install missing dependencies first (from the remote registry when set).
	var missing []requirement
	for _, dep := range m.DepOrder {
		c, err := ParseConstraint(m.Dependencies[dep])
		if err != nil {
			return nil, fmt.Errorf("dependency %s: %w", dep, err)
		}
		if installed, ok, err := idx.Installed(dep); err != nil {
			return nil, err
		} else if ok {
			if v, err := ParseVersion(installed.Version); err == nil && c.Satisfies(v) {
				continue
			}
		}
		missing = append(missing, requirement{name: dep, constraint: c, requiredBy: m.Package.Name + "@" + m.Package.Version})
	}
	if len(missing) > 0 {
		if in.client() == nil {
			var names []string
			for _, r := range missing {
				names = append(names, r.name)
			}
			return nil, fmt.Errorf("missing dependencies: %s\ninstall them first (apm install %s) or configure a registry with --registry",
				strings.Join(names, ", "), strings.Join(names, " "))
		}
		plan, err := in.resolveRemote(missing)
		if err != nil {
			return nil, err
		}
		if _, err := in.applyPlan(idx, plan); err != nil {
			return nil, err
		}
		idx, err = in.reg.LoadIndex()
		if err != nil {
			return nil, err
		}
	}

	// Full validation now that dependencies are resolvable.
	if _, err := ValidatePackageDir(dir); err != nil {
		return nil, err
	}

	staging, err := os.MkdirTemp(in.reg.TempDir(), "stage-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)
	if err := CopyTree(dir, staging); err != nil {
		return nil, err
	}
	pkg, err := in.finalizeInstall(idx, staging, m, "local:"+dir, nil)
	if err != nil {
		return nil, err
	}
	return pkg, nil
}

// CheckPackageStructure validates manifest, entry existence, source syntax and
// toolchain compatibility, without type checking the entry (which needs
// dependencies installed).
func CheckPackageStructure(dir string) (*Manifest, error) {
	m, err := ReadManifest(dir)
	if err != nil {
		return nil, err
	}
	entry := filepath.Join(dir, filepath.FromSlash(cleanRel(m.Package.Entry)))
	if !fileExists(entry) {
		return nil, fmt.Errorf("entry %q (from alca.toml) does not exist in %s", m.Package.Entry, dir)
	}
	if err := parseAllSources(dir); err != nil {
		return nil, err
	}
	if ok, err := AlcaCompatible(m.Package.Alca); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("package %s %s requires alca %q, running alca %s",
			m.Package.Name, m.Package.Version, m.Package.Alca, currentVersion())
	}
	return m, nil
}

// Remove uninstalls a package. It refuses while other installed packages depend
// on it unless force is set.
func (in *Installer) Remove(name string, force bool) ([]string, error) {
	idx, err := in.reg.LoadIndex()
	if err != nil {
		return nil, err
	}
	entry, ok := idx.Packages[name]
	if !ok {
		return nil, fmt.Errorf("package %q is not installed", name)
	}
	if !force {
		if deps := idx.Dependents(name); len(deps) > 0 {
			return nil, fmt.Errorf("cannot remove %s: %s depend(s) on it (use --force to remove anyway)",
				name, strings.Join(deps, ", "))
		}
	}
	var versions []string
	for v := range entry.Versions {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	if err := os.RemoveAll(filepath.Join(in.reg.PackagesDir(), name)); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	delete(idx.Packages, name)
	if err := in.reg.SaveIndex(idx); err != nil {
		return nil, err
	}
	return versions, nil
}

// List returns every installed package version, sorted by name and version.
func (in *Installer) List() ([]InstalledPackage, error) {
	idx, err := in.reg.LoadIndex()
	if err != nil {
		return nil, err
	}
	return indexToPackages(idx), nil
}

// Show returns the installed versions of one package.
func (in *Installer) Show(name string) ([]InstalledPackage, error) {
	idx, err := in.reg.LoadIndex()
	if err != nil {
		return nil, err
	}
	entry, ok := idx.Packages[name]
	if !ok {
		return nil, fmt.Errorf("package %q is not installed", name)
	}
	var out []InstalledPackage
	for _, v := range sortedVersionEntries(entry) {
		out = append(out, entryToPackage(v))
	}
	return out, nil
}

// CacheArchive stores a downloaded archive in the registry cache.
func (r *Registry) CacheArchive(name, version string, data []byte) error {
	return atomicWriteFile(filepath.Join(r.CacheDir(), name+"-"+version+".tar.gz"), data, 0o644)
}

// CachedArchive reads a cached archive, if present.
func (r *Registry) CachedArchive(name, version string) ([]byte, bool) {
	data, err := os.ReadFile(filepath.Join(r.CacheDir(), name+"-"+version+".tar.gz"))
	if err != nil {
		return nil, false
	}
	return data, true
}

func indexToPackages(idx *Index) []InstalledPackage {
	var out []InstalledPackage
	for _, name := range sortedKeys(idx.Packages) {
		entry := idx.Packages[name]
		for _, ve := range sortedVersionEntries(entry) {
			out = append(out, entryToPackage(ve))
		}
	}
	return out
}

func sortedVersionEntries(entry *PackageEntry) []*VersionEntry {
	var out []*VersionEntry
	for _, ve := range entry.Versions {
		out = append(out, ve)
	}
	sort.Slice(out, func(i, j int) bool {
		vi, erri := ParseVersion(out[i].Version)
		vj, errj := ParseVersion(out[j].Version)
		if erri != nil || errj != nil {
			return out[i].Version < out[j].Version
		}
		return vj.Less(vi)
	})
	return out
}

func entryToPackage(ve *VersionEntry) InstalledPackage {
	pkg := InstalledPackage{
		Name:         packageNameOf(ve),
		Version:      ve.Version,
		Description:  ve.Description,
		Entry:        ve.Entry,
		Alca:         ve.Alca,
		License:      ve.License,
		Path:         ve.Path,
		Origin:       ve.Origin,
		Dependencies: ve.Dependencies,
		InstalledAt:  ve.InstalledAt,
	}
	for _, key := range sortedKeys(ve.Artifacts) {
		pkg.Artifacts = append(pkg.Artifacts, *ve.Artifacts[key])
	}
	return pkg
}

// packageNameOf recovers the package name from an index entry: the explicit
// Name field when present, otherwise the install path "packages/<name>/<ver>".
func packageNameOf(ve *VersionEntry) string {
	if ve.Name != "" {
		return ve.Name
	}
	parts := strings.Split(filepath.ToSlash(ve.Path), "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
