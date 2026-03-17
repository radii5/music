package main

import (
	"os"

	"github.com/radii5/music/cmd"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		println(version)
		return
	}

	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		println("radii5 - Fast music downloader")
		println("Usage: radii5 <url> [flags]")
		println("")
		println("Flags:")
		println("  -f, --format    Audio format (mp3, flac, wav, m4a, opus)")
		println("  -o, --output    Output directory")
		println("  -t, --threads   Number of parallel download threads")
		println("  -v, --version   Print version")
		println("  -h, --help      Show help")
		return
	}

	cmd.Run(os.Args[1:])
}
