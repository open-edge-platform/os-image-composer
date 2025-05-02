package main

import (
    "fmt"
    "io/ioutil"
    "os"
	"path/filepath"
    "encoding/json"
	"go.uber.org/zap"
    "go.uber.org/zap/zapcore"
    "github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/pkgfetcher"
    "github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/validate"
    "github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/provider"
    _ "github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/provider/azurelinux3" // register provider
)

// temporary placeholder for configuration
// This should be replaced with a proper configuration struct
const (
	workers = 4
	destDir = "./downloads"
)

// setupLogger initializes a zap logger with development configuration.
// It sets the encoder to use color for levels and ISO8601 for time.
func setupLogger() (*zap.Logger, error) {
    cfg := zap.NewDevelopmentConfig()
    cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
    cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
    cfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
    return cfg.Build()
}

func main() {

    logger, err := setupLogger()
    if err != nil {
        panic(err)
    }
    defer logger.Sync()
    zap.ReplaceGlobals(logger)
    sugar := zap.S()

    // check for input JSON
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input.json>\n", os.Args[0])
		os.Exit(1)
	}
	configPath := os.Args[1]

	// read and validate JSON
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		sugar.Fatalf("reading file: %v", err)
	}
	if err := validate.ValidateJSON(data); err != nil {
		sugar.Fatalf("validation error: %v", err)
	}

    // print JSON config
	var cfg interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		sugar.Errorf("parsing JSON: %v", err)
	} else {
		pretty, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			sugar.Errorf("formatting JSON: %v", err)
		} else {
			sugar.Infof("loaded config:\n%s", string(pretty))
		}
	}

    // TODO: replace with actual provider initialization based on JSON config
    // For now, we will just use the azurelinux3 provider as an example
    
    // initialize provider
	p, ok := provider.Get("azurelinux3")
	if !ok {
		sugar.Fatalf("provider not found")
	}

	if err := p.Init(); err != nil {
		sugar.Fatalf("provider init: %v", err)
	}

	// fetch package list
	pkgs, err := p.Packages()
	if err != nil {
		sugar.Fatalf("getting packages: %v", err)
	}

	// extract URLs
	urls := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		urls[i] = pkg.URL
	}

	// start download
	absDest, err := filepath.Abs(destDir)
	if err != nil {
		sugar.Fatalf("invalid dest: %v", err)
	}
	sugar.Infof("downloading %d packages to %s", len(urls), absDest)
	if err := pkgfetcher.FetchPackages(urls, absDest, workers); err != nil {
		sugar.Fatalf("fetch failed: %v", err)
	}
	
	// start fetching
	sugar.Infof("starting download of %d packages into %s with %d workers", len(urls), absDest, workers)
	if err := pkgfetcher.FetchPackages(urls, absDest, workers); err != nil {
		sugar.Fatalf("fetching packages: %v", err)
	}
	sugar.Info("all downloads complete")
}