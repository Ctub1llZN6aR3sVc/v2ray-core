package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	syscall "syscall"

	"v2ray.com/core"
	"v2ray.com/core/common/cmdarg"
	"v2ray.com/core/common/platform"
	_ "v2ray.com/core/main/confloader/external"
	_ "v2ray.com/core/main/distro/all"
)

var (
	configFiles cmdarg.Arg // Configuration files to load
	configDir   string
	version     = flag.Bool("version", false, "Show current version of V2Ray.")
	testConfig  = flag.Bool("test", false, "Test configuration and exit.")
	format      = flag.String("format", "json", "Format of input file.")
)

func fileExists(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}

func dirExists(dir string) bool {
	if dir == "" {
		return false
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

func readConfDir(dirPath string) {
	confsDir, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read config directory: %s\n", err)
		return
	}
	for _, f := range confsDir {
		if f.IsDir() {
			continue
		}
		ext := filepath.Ext(f.Name())
		if strings.EqualFold(ext, ".json") || strings.EqualFold(ext, ".yaml") || strings.EqualFold(ext, ".yml") {
			configFiles.Set(filepath.Join(dirPath, f.Name()))
		}
	}
}

func getConfigFilePath() cmdarg.Arg {
	if dirExists(configDir) {
		readConfDir(configDir)
	}
	if len(configFiles) > 0 {
		return configFiles
	}

	if workingDir, err := os.Getwd(); err == nil {
		// Also check for config.yaml and config.yml in the working directory
		for _, name := range []string{"config.json", "config.yaml", "config.yml"} {
			defaultConfig := filepath.Join(workingDir, name)
			if fileExists(defaultConfig) {
				return cmdarg.Arg{defaultConfig}
			}
		}
	}

	if configFile := platform.GetConfigurationPath(); fileExists(configFile) {
		return cmdarg.Arg{configFile}
	}

	return nil
}

func startV2Ray() (core.Server, error) {
	configFiles := getConfigFilePath()

	config, err := core.LoadConfig(*format, configFiles[0], configFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to read config files: [%s] %w", configFiles.String(), err)
	}

	server, err := core.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	return server, nil
}

func main() {
	// Register config file flag
	flag.Var(&configFiles, "config", "Config file for V2Ray. Multiple assign is accepted (json, yaml).")
	flag.Var(&configFiles, "c", "Short alias of -config")
	flag.StringVar(&configDir, "confdir", "", "A directory with multiple config files, sorted and merged.")

	flag.Parse()

	printVersion()

	if *version {
		return
	}

	server, err := startV2Ray()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		// Configuration error — exit with code 23
		os.Exit(23)
	}

	if *testConfig {
		fmt.Println("Configuration OK.")
		return
	}

	if err := server.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to start:", err)
		os.Exit(1)
	}
	defer server.Close()

	// Handle OS signals for graceful shutdown
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)

	<-osSignals
	fmt.Println("V2Ray shutting down.")
}
