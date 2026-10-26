// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package apm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/compiler"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/typecheck"
	"github.com/alca-lang/alca/pkg/version"
	"github.com/alca-lang/alca/pkg/vm"
)

// Artifact format constants.
const (
	ArtifactFormat        = "alca-artifact"
	ArtifactFormatVersion = 1
)

// Sentinel errors for artifact loading. Callers fall back to source on all of
// them; none of them indicate a broken package.
var (
	// ErrArtifactUnsupported means the VM cannot compile this module (yet).
	ErrArtifactUnsupported = errors.New("module uses features the VM cannot compile")
	// ErrArtifactStale means the package sources changed after the artifact was built.
	ErrArtifactStale = errors.New("compiled artifact is stale")
	// ErrArtifactIncompatible means the artifact was built by a different
	// toolchain version or targets a different platform.
	ErrArtifactIncompatible = errors.New("compiled artifact is incompatible with this toolchain or platform")
)

// ArtifactMeta is the sidecar metadata of a compiled package artifact. It is
// stored next to the .bin file and shipped to registry nodes so clients can
// verify compatibility before using a build.
type ArtifactMeta struct {
	Format         string         `json:"format"`
	FormatVersion  int            `json:"format_version"`
	Name           string         `json:"name"`
	Version        string         `json:"version"`
	Entry          string         `json:"entry"`
	GOOS           string         `json:"goos"`
	GOARCH         string         `json:"goarch"`
	AlcaVersion    string         `json:"alca_version"`
	PayloadVersion uint32         `json:"payload_version"`
	SourceHash     string         `json:"source_hash"`
	Exports        map[string]int `json:"exports,omitempty"`
	CreatedAt      string         `json:"created_at"`
	SHA256         string         `json:"sha256"`
	Size           int64          `json:"size,omitempty"`
	Unsupported    string         `json:"unsupported,omitempty"` // reason when compile was skipped
}

// ArtifactPaths returns the relative (slash-separated) artifact and metadata
// paths for a platform inside a package directory.
func ArtifactPaths(goos, goarch string) (bin, meta string) {
	base := ".alca/artifacts/" + goos + "-" + goarch
	return base + ".bin", base + ".json"
}

// SourceHash fingerprints every .al source of a package directory so artifacts
// can be invalidated when the sources change.
func SourceHash(dir string) (string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir && (name == ".alca" || name == ".git" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".al") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		rel, err := filepath.Rel(dir, f)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		fileSum := sha256.Sum256(data)
		fmt.Fprintf(h, "%s\x00%s\n", filepath.ToSlash(rel), hex.EncodeToString(fileSum[:]))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// BuildArtifact compiles the package entry to VM bytecode and writes
// .alca/artifacts/<goos>-<goarch>.bin plus its metadata sidecar. It returns
// ErrArtifactUnsupported (wrapped) when the VM cannot compile the module; the
// caller then keeps the package source-only. Type errors are a real failure.
func BuildArtifact(dir string, m *Manifest, goos, goarch string) (*ArtifactMeta, error) {
	entryRel := cleanRel(m.Package.Entry)
	entry := filepath.Join(dir, filepath.FromSlash(entryRel))
	data, err := os.ReadFile(entry)
	if err != nil {
		return nil, fmt.Errorf("cannot read entry %s: %w", entryRel, err)
	}
	src := string(data)
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		first := p.SyntaxErrors()[0]
		return nil, fmt.Errorf("entry %s has syntax errors: %s", entryRel, first.Message)
	}
	checker := typecheck.NewChecker(entry, src)
	if errs := checker.Check(prog); len(errs) > 0 {
		return nil, fmt.Errorf("entry %s has %d type error(s): %s", entryRel, len(errs), errs[0].Message)
	}

	c := vm.NewCompiler()
	if err := c.Compile(prog); err != nil {
		var unsupported *vm.UnsupportedError
		if errors.As(err, &unsupported) {
			return nil, fmt.Errorf("%w: %s", ErrArtifactUnsupported, unsupported.Feature)
		}
		return nil, fmt.Errorf("cannot compile entry %s: %w", entryRel, err)
	}
	code := c.Bytecode()

	sum, err := SourceHash(dir)
	if err != nil {
		return nil, err
	}

	// Exports: top-level public declarations, mapped to their global slots so a
	// loader can expose them without re-parsing the source.
	globals := c.GlobalNames()
	slotByName := make(map[string]int, len(globals))
	for i, name := range globals {
		if name != "" {
			slotByName[name] = i
		}
	}
	exports := map[string]int{}
	for _, name := range publicDeclarations(prog) {
		if idx, ok := slotByName[name]; ok {
			exports[name] = idx
		}
	}

	bundle := &compiler.Bundle{Kind: compiler.KindBytecode, Name: entryRel, Code: code}
	encoded, err := bundle.Encode()
	if err != nil {
		return nil, fmt.Errorf("cannot serialize artifact: %w", err)
	}

	meta := &ArtifactMeta{
		Format:         ArtifactFormat,
		FormatVersion:  ArtifactFormatVersion,
		Name:           m.Package.Name,
		Version:        m.Package.Version,
		Entry:          entryRel,
		GOOS:           goos,
		GOARCH:         goarch,
		AlcaVersion:    version.Version,
		PayloadVersion: compiler.CurrentPayloadVersion,
		SourceHash:     sum,
		Exports:        exports,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		SHA256:         sha256Hex(encoded),
		Size:           int64(len(encoded)),
	}
	binRel, metaRel := ArtifactPaths(goos, goarch)
	if err := atomicWriteFile(filepath.Join(dir, filepath.FromSlash(binRel)), encoded, 0o644); err != nil {
		return nil, err
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := atomicWriteFile(filepath.Join(dir, filepath.FromSlash(metaRel)), metaData, 0o644); err != nil {
		return nil, err
	}
	return meta, nil
}

// LoadArtifact reads and verifies a compiled artifact for a platform. It
// returns ErrArtifactIncompatible, ErrArtifactStale or ErrArtifactUnsupported
// when the caller must fall back to the module sources.
func LoadArtifact(dir string, goos, goarch string) (*vm.Bytecode, *ArtifactMeta, error) {
	binRel, metaRel := ArtifactPaths(goos, goarch)
	metaData, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(metaRel)))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: metadata missing", ErrArtifactIncompatible)
	}
	var meta ArtifactMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, nil, fmt.Errorf("%w: corrupt metadata: %v", ErrArtifactIncompatible, err)
	}
	if meta.Format != ArtifactFormat || meta.FormatVersion != ArtifactFormatVersion {
		return nil, nil, fmt.Errorf("%w: unknown artifact format %q v%d", ErrArtifactIncompatible, meta.Format, meta.FormatVersion)
	}
	if meta.GOOS != goos || meta.GOARCH != goarch {
		return nil, nil, fmt.Errorf("%w: artifact targets %s/%s", ErrArtifactIncompatible, meta.GOOS, meta.GOARCH)
	}
	if meta.AlcaVersion != version.Version {
		return nil, nil, fmt.Errorf("%w: built by alca %s, running %s", ErrArtifactIncompatible, meta.AlcaVersion, version.Version)
	}
	if meta.PayloadVersion != compiler.CurrentPayloadVersion {
		return nil, nil, fmt.Errorf("%w: payload format %d, runtime supports %d", ErrArtifactIncompatible, meta.PayloadVersion, compiler.CurrentPayloadVersion)
	}
	current, err := SourceHash(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: cannot hash sources: %v", ErrArtifactStale, err)
	}
	if current != meta.SourceHash {
		return nil, nil, ErrArtifactStale
	}
	encoded, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(binRel)))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: artifact file missing", ErrArtifactIncompatible)
	}
	if got := sha256Hex(encoded); meta.SHA256 != "" && got != meta.SHA256 {
		return nil, nil, fmt.Errorf("%w: artifact checksum mismatch", ErrArtifactIncompatible)
	}
	bundle, err := compiler.DecodeBundle(encoded)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrArtifactIncompatible, err)
	}
	if bundle.Kind != compiler.KindBytecode || bundle.Code == nil {
		return nil, nil, fmt.Errorf("%w: artifact is not bytecode", ErrArtifactIncompatible)
	}
	return bundle.Code, &meta, nil
}

// publicDeclarations lists the top-level names a module exports: functions,
// structs and variables declared at file scope, matching the static checker's
// view of a module.
func publicDeclarations(prog *ast.Program) []string {
	var out []string
	for _, st := range prog.Statements {
		switch d := st.(type) {
		case *ast.FuncDeclaration:
			out = append(out, d.Name)
		case *ast.StructDeclaration:
			out = append(out, d.Name)
		case *ast.ClassDeclaration:
			out = append(out, d.Name)
		case *ast.VarDeclaration:
			out = append(out, d.Name.Value)
		}
	}
	return out
}
