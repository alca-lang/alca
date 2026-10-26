// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alca-lang/alca/pkg/android"
)

func cmdAndroid(args []string) int {
	if len(args) == 0 {
		printAndroidUsage()
		return 0
	}

	subcmd := args[0]
	switch subcmd {
	case "init", "new":
		return cmdAndroidInit(args[1:])
	case "build":
		return cmdAndroidBuild(args[1:])
	case "run":
		return cmdAndroidRun(args[1:])
	case "logs", "logcat":
		return cmdAndroidLogs(args[1:])
	case "check":
		return cmdAndroidCheck(args[1:])
	case "help", "-h", "--help":
		printAndroidUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown android subcommand %q\n\n", subcmd)
		printAndroidUsage()
		return 1
	}
}

func cmdInit(args []string) int {
	isAndroid := false
	var filtered []string
	for _, a := range args {
		if a == "--android" || a == "-android" || a == "-a" {
			isAndroid = true
		} else {
			filtered = append(filtered, a)
		}
	}

	if isAndroid {
		return cmdAndroidInit(filtered)
	}

	name := "myapp"
	if len(filtered) > 0 {
		name = filtered[0]
	}
	fmt.Printf("Initializing project %q (use 'alca init --android %s' for Android project)\n", name, name)
	return cmdAndroidInit(filtered)
}

func cmdAndroidInit(args []string) int {
	projectName := "MyApp"
	targetDir := "."

	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		targetDir = strings.TrimSpace(args[0])
		projectName = filepath.Base(targetDir)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating directory %s: %v\n", targetDir, err)
		return 1
	}

	if err := android.Scaffold(targetDir, projectName); err != nil {
		fmt.Fprintf(os.Stderr, "error initializing Android project: %v\n", err)
		return 1
	}

	cfg, _ := android.LoadConfig(targetDir)

	fmt.Printf("\n✓ Successfully initialized Alca Android project %q\n\n", projectName)
	fmt.Printf("Project Structure:\n")
	fmt.Printf("  %s/\n", targetDir)
	fmt.Printf("  ├── alca.toml                (package and android manifest configuration)\n")
	fmt.Printf("  ├── AndroidManifest.xml      (standard Android application manifest)\n")
	fmt.Printf("  ├── src/\n")
	fmt.Printf("  │   └── main.al              (Alca application entrypoint and logic)\n")
	fmt.Printf("  ├── res/\n")
	fmt.Printf("  │   ├── values/strings.xml   (application strings and metadata)\n")
	fmt.Printf("  │   └── mipmap/ic_launcher.png (application icon)\n")
	fmt.Printf("  └── assets/\n")
	fmt.Printf("      ├── index.html           (mobile UI interface template)\n")
	fmt.Printf("      ├── style.css            (mobile responsive styles)\n")
	fmt.Printf("      └── app.js               (JavaScript to Alca runtime bridge)\n\n")

	fmt.Printf("Configuration:\n")
	fmt.Printf("  Package:     %s\n", cfg.PackageName)
	fmt.Printf("  Label:       %s\n", cfg.Label)
	fmt.Printf("  Target SDK:  %d (Min SDK: %d)\n\n", cfg.TargetSDK, cfg.MinSDK)

	fmt.Printf("Next steps (zero Android Studio required!):\n")
	if targetDir != "." {
		fmt.Printf("  cd %s\n", targetDir)
	}
	fmt.Printf("  alca android build\n\n")
	return 0
}

func cmdAndroidBuild(args []string) int {
	var output string
	var projDir string

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-o" && i+1 < len(args) {
			output = args[i+1]
			i++
		} else if strings.HasPrefix(a, "-") {
			fmt.Fprintf(os.Stderr, "unknown option %s\n", a)
			return 1
		} else if projDir == "" {
			projDir = a
		}
	}

	if projDir == "" {
		projDir = "."
	}

	if err := android.ValidateProject(projDir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Println("Building Android package (.apk)...")

	apkPath, err := android.BuildAPK(android.BuildOptions{
		ProjectDir: projDir,
		OutputPath: output,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
		return 1
	}

	fi, err := os.Stat(apkPath)
	sizeKB := float64(0)
	if err == nil {
		sizeKB = float64(fi.Size()) / 1024.0
	}

	fmt.Printf("\n✓ Android APK built successfully!\n")
	fmt.Printf("  Output: %s (%.1f KB)\n", apkPath, sizeKB)
	fmt.Printf("  Status: Signed with Android debug certificate (ready to install)\n\n")
	fmt.Printf("To install on an Android device or emulator:\n")
	fmt.Printf("  adb install %s\n\n", apkPath)
	return 0
}

func cmdAndroidCheck(args []string) int {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	if err := android.ValidateProject(dir); err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		return 1
	}

	cfg, err := android.LoadConfig(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Valid Alca Android project at %s\n", dir)
	fmt.Printf("  Package:    %s\n", cfg.PackageName)
	fmt.Printf("  Label:      %s\n", cfg.Label)
	fmt.Printf("  Target SDK: %d (Min SDK: %d)\n", cfg.TargetSDK, cfg.MinSDK)
	fmt.Printf("  Activity:   %s\n", cfg.Activity)
	return 0
}

func cmdAndroidRun(args []string) int {
	projDir := "."
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		projDir = args[0]
	}

	apkPath, err := android.BuildAPK(android.BuildOptions{
		ProjectDir: projDir,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
		return 1
	}

	cfg, _ := android.LoadConfig(projDir)

	adbPath, err := exec.LookPath("adb")
	if err != nil {
		fmt.Printf("\n✓ APK built successfully: %s\n", apkPath)
		fmt.Println("Note: 'adb' tool not found in PATH to automatically deploy.")
		fmt.Printf("To install manually via USB or emulator:\n  adb install -r %s\n\n", apkPath)
		return 0
	}

	fmt.Printf("Installing %s to connected device/emulator...\n", filepath.Base(apkPath))
	cmd := exec.Command(adbPath, "install", "-r", apkPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "adb install failed: %v\n", err)
		return 1
	}

	activity := cfg.Activity
	if strings.HasPrefix(activity, ".") {
		activity = cfg.PackageName + activity
	}
	targetComponent := fmt.Sprintf("%s/%s", cfg.PackageName, activity)
	fmt.Printf("Starting application (%s)...\n", targetComponent)
	startCmd := exec.Command(adbPath, "shell", "am", "start", "-n", targetComponent)
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	_ = startCmd.Run()
	return 0
}

func cmdAndroidLogs(args []string) int {
	adbPath, err := exec.LookPath("adb")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: 'adb' not found in PATH")
		return 1
	}

	fmt.Println("Streaming Android logs (Ctrl+C to stop)...")
	cmd := exec.Command(adbPath, "logcat", "-s", "Alca:V", "Chromium:D", "*:E")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	return 0
}

func printAndroidUsage() {
	fmt.Println("Alca Android Build System (Zero-Toolchain APK Generator)")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  alca android <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init [name]          Initialize a new Android project structure")
	fmt.Println("  build [-o out.apk]   Compile the project into a signed Android APK")
	fmt.Println("  run [dir]            Build and install/launch on connected device via adb")
	fmt.Println("  logs                 Stream Android runtime logs (adb logcat)")
	fmt.Println("  check [dir]          Validate Android project structure and manifest")
	fmt.Println()
	fmt.Println("Zero Dependencies:")
	fmt.Println("  No Android Studio, Gradle, Java JDK, or external toolchain required.")
}
