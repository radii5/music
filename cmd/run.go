package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/fatih/color"
	"github.com/radii5/radii5/internal/downloader"
)

type Options struct {
	URL       string
	Format    string
	OutputDir string
	Threads   int
}

func defaultMusicDir() string {
	// Use OS music folder: ~/Music/radii5 downloads
	home, err := os.UserHomeDir()
	if err != nil {
		return "radii5 downloads"
	}
	return filepath.Join(home, "Music", "radii5 downloads")
}

func Run(args []string) {
	opts := parseArgs(args)

	if opts.URL == "" {
		color.Red("  ✗ No URL provided")
		fmt.Println("  Usage: radii5 <url>")
		os.Exit(1)
	}

	if err := downloader.Download(opts.URL, opts.Format, opts.OutputDir, opts.Threads); err != nil {
		color.Red("✗ %v", err)
		os.Exit(1)
	}
}

func parseArgs(args []string) Options {
	opts := Options{
		Format:    "mp3",
		OutputDir: defaultMusicDir(),
		Threads:   8,
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format", "-f":
			if i+1 < len(args) {
				i++
				opts.Format = args[i]
			}
		case "--output", "-o":
			if i+1 < len(args) {
				i++
				opts.OutputDir = args[i]
			}
		case "--threads", "-t":
			if i+1 < len(args) {
				i++
				if n, err := strconv.Atoi(args[i]); err == nil {
					opts.Threads = n
				}
			}
		default:
			// First non-flag arg is the URL
			if opts.URL == "" && len(args[i]) > 0 && args[i][0] != '-' {
				opts.URL = args[i]
			} else if args[i][0] == '-' {
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			}
		}
	}

	return opts
}
