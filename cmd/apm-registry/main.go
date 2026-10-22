// Command apm-registry runs an Alca package registry node: it stores published
// libraries and their compiled builds, serves the APM HTTP API, and
// authenticates publishers with bearer tokens.
//
// Quick start:
//
//	apm-registry --init-config apm-registry.json   # write an example config
//	apm-registry --config apm-registry.json        # run the node
//
// See docs/apm-registry.md for deployment instructions.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alca-lang/alca/pkg/registry"
	"github.com/alca-lang/alca/pkg/version"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("apm-registry", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "path to a JSON configuration file")
	listen := fs.String("listen", "", "listen address (overrides the config, e.g. 0.0.0.0:8787)")
	port := fs.Int("p", 0, "port number (overrides config listen address)")
	dataDir := fs.String("data", "", "storage directory (overrides the config)")
	tlsCert := fs.String("tls-cert", "", "TLS certificate file (enables HTTPS)")
	tlsKey := fs.String("tls-key", "", "TLS private key file")
	readOnly := fs.Bool("read-only", false, "serve downloads but refuse writes")
	initConfig := fs.String("init-config", "", "write an example configuration file and exit")
	showVersion := fs.Bool("version", false, "print the version and exit")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: apm-registry [--config file.json] [--listen addr] [-p port] [--data dir] [--tls-cert file --tls-key file] [--read-only]")
		fmt.Fprintln(os.Stderr, "       apm-registry --init-config file.json")
		fmt.Fprintln(os.Stderr, "\nAn Alca registry node stores published libraries and serves the APM API.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Printf("apm-registry %s\n", version.Version)
		return 0
	}
	if *initConfig != "" {
		if err := writeExampleConfig(*initConfig); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Printf("Wrote example config to %s\n", *initConfig)
		fmt.Println("Edit the tokens, listen address and data_dir, then start the node with:")
		fmt.Printf("  apm-registry --config %s\n", *initConfig)
		fmt.Println()
		fmt.Println("Or start with custom port:")
		fmt.Printf("  apm-registry --config %s -p 9977\n", *initConfig)
		fmt.Println()
		fmt.Println("With allow_registration enabled (the example default), users create")
		fmt.Println("accounts and one-time tokens themselves:")
		fmt.Println("  apm register <username> <email> --registry http://host:port")
		return 0
	}

	cfg := registry.DefaultConfig()
	if *configPath != "" {
		loaded, err := registry.LoadConfig(*configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		cfg = loaded
	}
	// Handle -p flag: if port is specified, override the listen address
	if *port > 0 {
		cfg.Listen = "0.0.0.0:" + fmt.Sprintf("%d", *port)
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if *tlsCert != "" {
		cfg.TLSCert = *tlsCert
	}
	if *tlsKey != "" {
		cfg.TLSKey = *tlsKey
	}
	if *readOnly {
		cfg.ReadOnly = true
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: levelFromName(cfg.LogLevel)}))
	srv, err := registry.New(cfg, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("registry stopped", "error", err)
		return 1
	}
	return 0
}

func levelFromName(name string) slog.Level {
	switch name {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func writeExampleConfig(path string) error {
	example := registry.DefaultConfig()
	// The quick start serves a self-service registry: anyone who registers gets
	// an account and a one-time token. Set this to false to close registration
	// and keep only the operator tokens below.
	example.AllowRegistration = true
	example.Tokens = []registry.Token{
		{Token: "replace-with-a-long-random-token", Author: "operator", Scopes: []string{"publish"}},
	}
	example.LogLevel = "info"
	data, err := json.MarshalIndent(example, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if existing, err := os.Stat(path); err == nil && !existing.IsDir() {
		return fmt.Errorf("%s already exists; refusing to overwrite", path)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return nil
}
