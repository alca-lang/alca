// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Command apm is the Alca package manager.
//
// It installs libraries from local sources (-u), from Alca registry nodes, or
// from already-installed dependencies, maintains the local system registry that
// `use <pkg>` resolves against, and shows what is installed.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/alca-lang/alca/pkg/apm"
	"github.com/alca-lang/alca/pkg/version"
)

const usage = `APM - the Alca package manager

Usage:
  apm init [name]                         initialize a new Alca library with alca.toml
  apm install [<package>[@<version>] ...] install libraries and their dependencies
  apm install -u <directory>              install a library from local sources
  apm remove <package> [--force]          uninstall a library
  apm show [package]                      show installed libraries
  apm register [username] [email]         create an account; prints a one-time token
  apm login [username]                    sign in and print a new one-time token
  apm whoami                              show the signed-in account
  apm logout                              revoke the saved token on the node
  apm version                             print the APM version
  apm help                                show this help

Options:
  --registry <url>   registry node URL (default: DEFAULT_REGISTRY)
  --home <dir>       Alca home directory (default: $ALCA_HOME or ~/.alca)
  --password <pw>    account password for register/login (or $APM_PASSWORD; else prompt)
  --allow-insecure-http   send credentials over plain http (your own IP:port node)
  --force            override safety checks (broken dependents, removal)
  --insecure         skip TLS certificate verification (development only)

Installed libraries are importable directly:
  use http;
  println(http.get("https://example.com").status);
`

// usageText renders the help text, naming the compiled-in registry node when
// this build carries one.
func usageText() string {
	def := "$ALCA_REGISTRY"
	if u := apm.DefaultRegistryURL(); u != "" {
		def = "$ALCA_REGISTRY or default registry (built in)"
	}
	return strings.Replace(usage, "DEFAULT_REGISTRY", def, 1)
}

func displayRegistry(raw string) string {
	if raw == apm.DefaultRegistryURL() || strings.Contains(raw, "91.92.34.94") {
		return "registry"
	}
	return apm.StripServerInfo(raw)
}

type cliOptions struct {
	home       string
	registry   string
	password   string
	force      bool
	insecure   bool
	token      string
	local      string
	allowHTTP  bool
	targets    []string
	positional []string
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Print(usageText())
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Print(usageText())
		return 0
	case "version", "-v", "--version":
		fmt.Printf("apm %s (alca %s)\n", version.Version, version.Version)
		return 0
	case "init":
		return cmdInit(args[1:])
	case "install":
		return cmdInstall(args[1:])
	case "remove", "uninstall":
		return cmdRemove(args[1:])
	case "show", "list":
		return cmdShow(args[1:])
	case "register", "signup":
		return cmdRegister(args[1:])
	case "login", "signin":
		return cmdLogin(args[1:])
	case "logout":
		return cmdLogout(args[1:])
	case "whoami", "account":
		return cmdWhoami(args[1:])
	case "publish":
		return cmdPublish(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		fmt.Print(usageText())
		return 2
	}
}

func parseOptions(args []string, allowLocal bool) (*cliOptions, error) {
	opts := &cliOptions{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--registry" && i+1 < len(args):
			opts.registry = args[i+1]
			i++
		case a == "--home" && i+1 < len(args):
			opts.home = args[i+1]
			i++
		case a == "--token" && i+1 < len(args):
			opts.token = args[i+1]
			i++
		case a == "--password" && i+1 < len(args):
			opts.password = args[i+1]
			i++
		case a == "--target" && i+1 < len(args):
			opts.targets = append(opts.targets, args[i+1])
			i++
		case (a == "-u" || a == "--local") && allowLocal && i+1 < len(args):
			opts.local = args[i+1]
			i++
		case a == "--force":
			opts.force = true
		case a == "--insecure":
			opts.insecure = true
		case a == "--allow-insecure-http":
			opts.allowHTTP = true
		case strings.HasPrefix(a, "-"):
			return nil, fmt.Errorf("unknown option %q", a)
		default:
			opts.positional = append(opts.positional, a)
		}
	}
	opts.registry = apm.ResolveRegistryURL(opts.registry)
	if opts.token == "" {
		opts.token = strings.TrimSpace(os.Getenv(apm.EnvToken))
	}
	return opts, nil
}

func installerFor(opts *cliOptions) (*apm.Installer, error) {
	reg, err := apm.NewRegistry(opts.home)
	if err != nil {
		return nil, err
	}
	return apm.NewInstaller(reg, apm.InstallOptions{
		RegistryURL: opts.registry,
		InsecureTLS: opts.insecure,
		Force:       opts.force,
		Out:         os.Stdout,
	}), nil
}

func cmdInit(args []string) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	name := filepath.Base(wd)
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		name = strings.TrimSpace(args[0])
	}
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")

	manifestPath := filepath.Join(wd, "alca.toml")
	if _, err := os.Stat(manifestPath); err == nil {
		fmt.Fprintln(os.Stderr, "error: alca.toml already exists in this directory")
		return 1
	}

	manifestContent := fmt.Sprintf(`[package]
name = "%s"
version = "0.1.0"
description = "A new Alca package"
entry = "src/main.al"
alca = "%s"

[dependencies]
`, name, version.Version)

	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "error writing alca.toml:", err)
		return 1
	}

	srcDir := filepath.Join(wd, "src")
	_ = os.MkdirAll(srcDir, 0755)

	mainFile := filepath.Join(srcDir, "main.al")
	if _, err := os.Stat(mainFile); os.IsNotExist(err) {
		starter := fmt.Sprintf(`// %s main entry point
func main() -> void:
    println("Hello from %s!");
`, name, name)
		_ = os.WriteFile(mainFile, []byte(starter), 0644)
	}

	fmt.Printf("Initialized Alca package %q in %s (alca.toml)\n", name, wd)
	return 0
}

func cmdInstall(args []string) int {
	opts, err := parseOptions(args, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if opts.local == "" && len(opts.positional) == 0 {
		wd, _ := os.Getwd()
		manifestPath := filepath.Join(wd, "alca.toml")
		if data, err := os.ReadFile(manifestPath); err == nil {
			m, parseErr := apm.ParseManifest(data)
			if parseErr != nil {
				fmt.Fprintln(os.Stderr, "error parsing alca.toml:", parseErr)
				return 1
			}
			if len(m.Dependencies) == 0 {
				fmt.Println("alca.toml has no dependencies.")
				return 0
			}
			for depName, constraint := range m.Dependencies {
				opts.positional = append(opts.positional, fmt.Sprintf("%s@%s", depName, constraint))
			}
		} else {
			fmt.Fprintln(os.Stderr, "usage: apm install [<package>[@<version>] ...] | apm install -u <directory>")
			return 2
		}
	}
	if opts.local != "" && len(opts.positional) > 0 {
		fmt.Fprintln(os.Stderr, "error: -u takes a single directory; install multiple packages in separate commands")
		return 2
	}
	in, err := installerFor(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	if opts.local != "" {
		abs, err := filepath.Abs(opts.local)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Printf("Installing %s from local sources...\n", abs)
		pkg, err := in.InstallLocal(abs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		printInstalled([]apm.InstalledPackage{*pkg})
		return 0
	}

	fmt.Printf("Resolving %s...\n", strings.Join(opts.positional, ", "))
	pkgs, err := in.Install(opts.positional...)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "\033[") {
			fmt.Fprintln(os.Stderr, msg)
		} else {
			fmt.Fprintln(os.Stderr, "error:", apm.StripServerInfo(msg))
		}
		return 1
	}
	printInstalled(pkgs)

	// Update apm.lock if running in an Alca project
	if wd, err := os.Getwd(); err == nil {
		if lockPath, inProject := apm.FindLockfilePath(wd); inProject {
			lock, _ := apm.ReadLockfile(lockPath)
			if lock == nil {
				lock = apm.NewLockfile()
			}
			lock.UpdateFromInstalled(pkgs)
			if err := lock.Write(lockPath); err == nil {
				fmt.Printf("Locked %d package(s) in %s\n", len(lock.Packages), filepath.Base(lockPath))
			}
		}
	}

	return 0
}

func printInstalled(pkgs []apm.InstalledPackage) {
	if len(pkgs) == 0 {
		fmt.Println("Nothing to do; everything is already installed.")
		return
	}
	fmt.Println("Installed:")
	for _, p := range pkgs {
		state := ""
		if p.AlreadyExists {
			state = " (already installed)"
		}
		artifact := "sources"
		if len(p.Artifacts) > 0 {
			var parts []string
			for _, a := range p.Artifacts {
				parts = append(parts, a.GOOS+"/"+a.GOARCH)
			}
			artifact = "compiled " + strings.Join(parts, ", ")
		}
		fmt.Printf("  %s %s%s [%s]\n", p.Name, p.Version, state, artifact)
		if len(p.Dependencies) > 0 {
			fmt.Printf("      dependencies: %s\n", formatDeps(p.Dependencies))
		}
	}
}

func formatDeps(deps map[string]string) string {
	names := make([]string, 0, len(deps))
	for name := range deps {
		names = append(names, name)
	}
	// small and dependency-free sort
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	var out []string
	for _, name := range names {
		c := deps[name]
		if c == "" || c == "*" {
			out = append(out, name)
		} else {
			out = append(out, name+" "+c)
		}
	}
	return strings.Join(out, ", ")
}

func cmdRemove(args []string) int {
	opts, err := parseOptions(args, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if len(opts.positional) != 1 {
		fmt.Fprintln(os.Stderr, "usage: apm remove <package> [--force]")
		return 2
	}
	name := opts.positional[0]
	if !apm.ValidName(name) {
		fmt.Fprintf(os.Stderr, "error: invalid package name %q\n", name)
		return 2
	}
	in, err := installerFor(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	versions, err := in.Remove(name, opts.force)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("Removed %s (%s)\n", name, strings.Join(versions, ", "))
	return 0
}

func cmdShow(args []string) int {
	opts, err := parseOptions(args, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	in, err := installerFor(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if len(opts.positional) > 1 {
		fmt.Fprintln(os.Stderr, "usage: apm show [package]")
		return 2
	}
	if len(opts.positional) == 1 {
		pkgs, err := in.Show(opts.positional[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		printDetails(pkgs)
		return 0
	}
	pkgs, err := in.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if len(pkgs) == 0 {
		fmt.Println("No libraries installed.")
		fmt.Println("Install one with: apm install <package>  or  apm install -u <directory>")
		return 0
	}
	fmt.Printf("%d installed library version(s):\n\n", len(pkgs))
	current := ""
	for _, p := range pkgs {
		if p.Name != current {
			current = p.Name
			fmt.Printf("%s\n", p.Name)
		}
		fmt.Printf("  %s\n", p.Version)
		printDetails([]apm.InstalledPackage{p})
	}
	return 0
}

func printDetails(pkgs []apm.InstalledPackage) {
	for _, p := range pkgs {
		if p.Description != "" {
			fmt.Printf("    description: %s\n", p.Description)
		}
		fmt.Printf("    entry: %s\n", p.Entry)
		if p.Alca != "" {
			fmt.Printf("    requires alca: %s\n", p.Alca)
		}
		if p.License != "" {
			fmt.Printf("    license: %s\n", p.License)
		}
		if p.Path != "" {
			fmt.Printf("    path: %s\n", p.Path)
		}
		if p.InstalledAt != "" {
			fmt.Printf("    installed: %s\n", p.InstalledAt)
		}
		if p.Origin != "" {
			orig := p.Origin
			if strings.HasPrefix(orig, "remote:") {
				orig = "remote"
			}
			fmt.Printf("    origin: %s\n", orig)
		}
		if len(p.Dependencies) > 0 {
			fmt.Printf("    dependencies: %s\n", formatDeps(p.Dependencies))
		}
		for _, a := range p.Artifacts {
			fmt.Printf("    artifact: %s/%s (alca %s, payload v%d)\n", a.GOOS, a.GOARCH, a.AlcaVersion, a.PayloadVersion)
		}
	}
}

// ---------------------------------------------------------------------------
// Accounts
// ---------------------------------------------------------------------------

// requireRegistry resolves the registry URL and enforces the credential-safety
// policy: secrets only travel over https, unless the address is loopback or the
// user explicitly allowed plain http for a node on an IP and port.
func requireRegistry(opts *cliOptions) (string, error) {
	registryURL := apm.ResolveRegistryURL(opts.registry)
	if err := apm.CheckSecureRegistryURL(registryURL, opts.allowHTTP); err != nil {
		return "", err
	}
	return registryURL, nil
}

func cmdRegister(args []string) int {
	opts, err := parseOptions(args, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if len(opts.positional) > 2 {
		fmt.Fprintln(os.Stderr, "usage: apm register [username] [email] [--password <pw>] [--registry <url>]")
		return 2
	}
	registryURL, err := requireRegistry(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	username := ""
	if len(opts.positional) > 0 {
		username = opts.positional[0]
	}
	email := ""
	if len(opts.positional) > 1 {
		email = opts.positional[1]
	}
	username, err = promptValue("Username: ", username)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	email, err = promptValue("Email: ", email)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	password, err := promptPassword(opts, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	client := apm.NewClient(registryURL, apm.ClientOptions{InsecureTLS: opts.insecure})
	res, err := client.Register(username, email, password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("Registered %s on %s\n\n", res.Username, displayRegistry(registryURL))
	printTokenAndSave(opts, registryURL, res)
	return 0
}

func cmdLogin(args []string) int {
	opts, err := parseOptions(args, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if len(opts.positional) > 1 {
		fmt.Fprintln(os.Stderr, "usage: apm login [username] [--password <pw>] [--registry <url>]")
		return 2
	}
	registryURL, err := requireRegistry(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	username := ""
	if len(opts.positional) > 0 {
		username = opts.positional[0]
	}
	username, err = promptValue("Username: ", username)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	password, err := promptPassword(opts, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	client := apm.NewClient(registryURL, apm.ClientOptions{InsecureTLS: opts.insecure})
	res, err := client.Login(username, password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("Signed in as %s on %s\n\n", res.Username, displayRegistry(registryURL))
	printTokenAndSave(opts, registryURL, res)
	return 0
}

func cmdWhoami(args []string) int {
	opts, err := parseOptions(args, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if len(opts.positional) > 0 {
		fmt.Fprintln(os.Stderr, "usage: apm whoami [--registry <url>]")
		return 2
	}
	registryURL, err := requireRegistry(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	cred, err := savedCredential(opts, registryURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	client := apm.NewClient(registryURL, apm.ClientOptions{Token: cred.Token, InsecureTLS: opts.insecure})
	acct, err := client.Account()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if acct.Operator {
		fmt.Printf("node operator token %q on %s (not an account)\n", acct.Username, displayRegistry(registryURL))
		return 0
	}
	fmt.Printf("%s <%s> on %s\n", acct.Username, acct.Email, displayRegistry(registryURL))
	if acct.CreatedAt != "" {
		fmt.Printf("  account created: %s\n", acct.CreatedAt)
	}
	fmt.Printf("  tokens: %d\n", len(acct.Tokens))
	for _, t := range acct.Tokens {
		used := t.LastUsedAt
		if used == "" {
			used = "never"
		}
		label := t.Label
		if label == "" {
			label = "token"
		}
		fmt.Printf("    %s… (%s, created %s, last used %s)\n", t.Prefix, label, t.CreatedAt, used)
	}
	return 0
}

func cmdLogout(args []string) int {
	opts, err := parseOptions(args, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if len(opts.positional) > 0 {
		fmt.Fprintln(os.Stderr, "usage: apm logout [--registry <url>]")
		return 2
	}
	registryURL, err := requireRegistry(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	creds, err := apm.LoadCredentials(opts.home)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	cred, ok := creds.Get(registryURL)
	if !ok {
		fmt.Printf("No saved token for %s.\n", displayRegistry(registryURL))
		return 0
	}
	client := apm.NewClient(registryURL, apm.ClientOptions{Token: cred.Token, InsecureTLS: opts.insecure})
	if err := client.Logout(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: the node did not confirm the revocation: %v\n", err)
	}
	if err := creds.Delete(registryURL); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("Signed out %s from %s.\n", cred.Username, displayRegistry(registryURL))
	return 0
}

// savedCredential returns the stored account token for a registry node.
func savedCredential(opts *cliOptions, registryURL string) (apm.Credential, error) {
	creds, err := apm.LoadCredentials(opts.home)
	if err != nil {
		return apm.Credential{}, err
	}
	cred, ok := creds.Get(registryURL)
	if !ok {
		return apm.Credential{}, fmt.Errorf("not signed in to %s; run `apm register` or `apm login` first", registryURL)
	}
	return cred, nil
}

// printTokenAndSave prints a one-time token and stores it for later use.
func printTokenAndSave(opts *cliOptions, registryURL string, res *apm.AuthResponse) {
	fmt.Println("Your token (shown only once — save it now):")
	fmt.Printf("  %s\n\n", res.Token)
	creds, err := apm.LoadCredentials(opts.home)
	if err == nil {
		err = creds.Set(registryURL, res.Username, res.Token)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot save the token: %v\n", err)
		fmt.Fprintln(os.Stderr, "store the token above yourself: it will not be shown again")
		return
	}
	fmt.Printf("Saved to %s; publishing uses it automatically.\n", creds.Path())
}

// promptValue asks for a value that was not supplied on the command line.
func promptValue(label, value string) (string, error) {
	if v := strings.TrimSpace(value); v != "" {
		return v, nil
	}
	fmt.Print(label)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return "", fmt.Errorf("cannot read %s: %w", strings.ToLower(strings.TrimSuffix(label, ":")), err)
	}
	result := strings.TrimSpace(line)
	if result == "" {
		return "", fmt.Errorf("%s must not be empty", strings.ToLower(strings.TrimSuffix(label, ":")))
	}
	return result, nil
}

// promptPassword reads the account password from --password, $APM_PASSWORD or
// the terminal (without echoing when stdin is a terminal). Registration asks
// for the password twice when it was typed interactively.
func promptPassword(opts *cliOptions, confirm bool) (string, error) {
	password := opts.password
	if strings.TrimSpace(password) == "" {
		password = strings.TrimSpace(os.Getenv(apm.EnvPassword))
	}
	if strings.TrimSpace(password) != "" {
		return password, nil
	}
	password, err := readSecret("Password: ")
	if err != nil {
		return "", err
	}
	if password == "" {
		return "", fmt.Errorf("password must not be empty")
	}
	if confirm {
		again, err := readSecret("Repeat password: ")
		if err != nil {
			return "", err
		}
		if again != password {
			return "", fmt.Errorf("passwords do not match")
		}
	}
	return password, nil
}

// readSecret reads one line without echoing it when stdin is a terminal.
func readSecret(label string) (string, error) {
	fmt.Print(label)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		data, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// cmdPublish implements the publication path. It is intentionally not part of
// the advertised command set yet: the code and registry API are ready, and
// operators enable it per invocation with APM_ENABLE_PUBLISH=1.
func cmdPublish(args []string) int {
	if strings.TrimSpace(os.Getenv(apm.EnvEnablePublish)) != "1" {
		fmt.Fprintln(os.Stderr, "error: publishing is not enabled in this release.")
		fmt.Fprintf(os.Stderr, "It is prepared and gated: set %s=1 to use `apm publish`.\n", apm.EnvEnablePublish)
		return 2
	}
	opts, err := parseOptions(args, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	dir := "."
	if len(opts.positional) == 1 {
		dir = opts.positional[0]
	} else if len(opts.positional) > 1 {
		fmt.Fprintln(os.Stderr, "usage: apm publish [directory] [--registry <url>] [--token <token>] [--target os/arch]")
		return 2
	}
	token := opts.token
	if token == "" && opts.registry != "" {
		if creds, err := apm.LoadCredentials(opts.home); err == nil {
			if cred, ok := creds.Get(opts.registry); ok {
				token = cred.Token
				fmt.Printf("Using the saved token of %s.\n", cred.Username)
			}
		}
	}
	result, err := apm.Publish(apm.PublishOptions{
		Dir:               dir,
		RegistryURL:       opts.registry,
		Token:             token,
		InsecureTLS:       opts.insecure,
		AllowInsecureHTTP: opts.allowHTTP,
		Targets:           opts.targets,
		Out:               os.Stdout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	status := "Published"
	if result.AlreadyStored {
		status = "Already published"
	}
	fmt.Printf("%s %s %s (%s)\n", status, result.Name, result.Version, shortHash(result.SHA256))
	for _, a := range result.Artifacts {
		fmt.Printf("  build: %s/%s (%s)\n", a.GOOS, a.GOARCH, shortHash(a.SHA256))
	}
	return 0
}

func shortHash(h string) string {
	if len(h) > 12 {
		return "sha256:" + h[:12]
	}
	return "sha256:" + h
}
