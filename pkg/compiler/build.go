package compiler

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Binary format constants.
var (
	// PayloadMagic is the 16-byte identifier appended to compiled Alca binaries.
	PayloadMagic = []byte("ALCA_EXEC_V2\x00\x00\x00\x00")
	// FooterLen is 8 bytes payload size + 16 bytes magic.
	FooterLen = len(PayloadMagic) + 8
)

// CurrentPayloadVersion is the payload envelope version.
const CurrentPayloadVersion uint32 = 2

// VerifyMagic reports whether footer ends with the Alca payload marker.
func VerifyMagic(footer []byte) bool {
	if len(footer) < FooterLen {
		return false
	}
	return bytes.Equal(footer[8:FooterLen], PayloadMagic)
}

// StripPayload removes a previously appended payload so a built program can itself act as a stub.
func StripPayload(exe []byte) []byte {
	if len(exe) >= FooterLen && VerifyMagic(exe[len(exe)-FooterLen:]) {
		size := binary.LittleEndian.Uint64(exe[len(exe)-FooterLen:])
		if int64(size)+int64(FooterLen) <= int64(len(exe)) {
			return exe[:len(exe)-int(size)-FooterLen]
		}
	}
	return exe
}

// ExtractBundle reads the payload from a standalone executable.
func ExtractBundle(exePath string) (*Bundle, error) {
	f, err := os.Open(exePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open executable %q: %w", exePath, err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size() < int64(FooterLen) {
		return nil, fmt.Errorf("file is too small to be a compiled Alca executable")
	}
	footer := make([]byte, FooterLen)
	if _, err := f.ReadAt(footer, fi.Size()-int64(FooterLen)); err != nil {
		return nil, fmt.Errorf("cannot read executable footer: %w", err)
	}
	if !VerifyMagic(footer) {
		return nil, fmt.Errorf("file is not a compiled Alca executable (missing magic marker)")
	}
	size := binary.LittleEndian.Uint64(footer[:8])
	if int64(size)+int64(FooterLen) > fi.Size() {
		return nil, fmt.Errorf("invalid payload size recorded in footer")
	}
	data := make([]byte, size)
	if _, err := f.ReadAt(data, fi.Size()-int64(FooterLen)-int64(size)); err != nil {
		return nil, fmt.Errorf("cannot read payload bytes: %w", err)
	}
	return DecodeBundle(data)
}

// Target is an operating system / architecture pair ("linux/amd64").
type Target struct{ OS, Arch string }

func (t Target) String() string { return t.OS + "/" + t.Arch }

// HostTarget is the platform this binary runs on.
func HostTarget() Target { return Target{runtime.GOOS, runtime.GOARCH} }

// SupportedTargets lists the platforms a runtime can be built for.
var SupportedTargets = []Target{
	{"linux", "amd64"}, {"linux", "arm64"},
	{"darwin", "amd64"}, {"darwin", "arm64"},
	{"windows", "amd64"}, {"windows", "arm64"},
}

// ParseTarget parses "os/arch" (also accepts "os-arch").
func ParseTarget(s string) (Target, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), "-", "/")
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return Target{}, fmt.Errorf("invalid target %q: expected os/arch, e.g. linux/amd64", s)
	}
	t := Target{parts[0], parts[1]}
	for _, ok := range SupportedTargets {
		if ok == t {
			return t, nil
		}
	}
	return Target{}, fmt.Errorf("unsupported target %q (supported: %s)", t, targetList())
}

func targetList() string {
	var s []string
	for _, t := range SupportedTargets {
		s = append(s, t.String())
	}
	return strings.Join(s, ", ")
}

// RuntimeName is the file name of the runtime stub for a target.
func RuntimeName(t Target) string {
	name := fmt.Sprintf("alca-%s-%s", t.OS, t.Arch)
	if t.OS == "windows" {
		name += ".exe"
	}
	return name
}

// ExeName appends the platform's executable suffix.
func ExeName(base string, t Target) string {
	if t.OS == "windows" && !strings.HasSuffix(strings.ToLower(base), ".exe") {
		return base + ".exe"
	}
	return base
}

// FindRuntime locates the runtime stub for target. The host platform needs no extra
// files (the running alca is the stub). Other platforms need a runtime built with
// `make runtimes` / scripts/build-runtimes.sh, searched in $ALCA_RUNTIMES, the
// "runtimes" folder next to the alca binary, and ~/.alca/runtimes.
func FindRuntime(t Target, explicit string) (string, error) {
	if explicit != "" {
		if !fileExists(explicit) {
			return "", fmt.Errorf("runtime %q does not exist", explicit)
		}
		return explicit, nil
	}
	var dirs []string
	if d := os.Getenv("ALCA_RUNTIMES"); d != "" {
		dirs = append(dirs, d)
	}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "runtimes"), filepath.Dir(exe))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".alca", "runtimes"))
	}
	dirs = append(dirs, "runtimes")
	for _, d := range dirs {
		p := filepath.Join(d, RuntimeName(t))
		if fileExists(p) {
			return p, nil
		}
	}
	if t == HostTarget() {
		if exe, err := os.Executable(); err == nil && fileExists(exe) {
			return exe, nil
		}
	}
	return "", fmt.Errorf("no Alca runtime for %s found.\n  build runtimes once with:  make runtimes   (or scripts/build-runtimes.sh)\n  then point to them with:   ALCA_RUNTIMES=./runtimes alca build ... --target %s\n  looked for %q in: %s", t, t, RuntimeName(t), strings.Join(dirs, ", "))
}

// Build writes stub + payload + footer to outputPath.
func Build(b *Bundle, stubPath, outputPath string) error {
	stub, err := os.ReadFile(stubPath)
	if err != nil {
		return fmt.Errorf("failed to read runtime %q: %w", stubPath, err)
	}
	stub = StripPayload(stub)
	payload, err := b.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}
	var out bytes.Buffer
	out.Grow(len(stub) + len(payload) + FooterLen)
	out.Write(stub)
	out.Write(payload)
	_ = binary.Write(&out, binary.LittleEndian, uint64(len(payload)))
	out.Write(PayloadMagic)
	if err := os.WriteFile(outputPath, out.Bytes(), 0o755); err != nil {
		return fmt.Errorf("failed to write %q: %w", outputPath, err)
	}
	return nil
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
