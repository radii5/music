package main
import (
    "fmt"
    "os"
    "github.com/fatih/color"
    "github.com/radii5/music/cmd"
)
var version = "0.2.0"
func main() {
    if len(os.Args) < 2 {
        printUsage()
        os.Exit(0)
    }
    arg := os.Args[1]
    switch arg {
    case "--version", "-v":
        fmt.Printf("radii5 v%s\n", version)
    case "--help", "-h":
        printUsage()
    default:
        cmd.Run(os.Args[1:])
    }
}
func printUsage() {
    cyan := color.New(color.FgCyan, color.Bold)
    gray := color.New(color.FgHiBlack)
    cyan.Println("radii5 v" + version)
    fmt.Println()
    fmt.Println("  Fast music downloader — built for speed, not bloat.")
    fmt.Println()
    color.New(color.FgWhite, color.Bold).Println("Usage:")
    fmt.Println("  radii5 <url>                  Download audio from URL")
    fmt.Println("  radii5 <url> --format flac    Download as FLAC")
    fmt.Println("  radii5 <url> --output ~/Music  Set output directory")
    fmt.Println("  radii5 <url> --threads 16     Set parallel chunks (default: 8)")
    fmt.Println()
    color.New(color.FgWhite, color.Bold).Println("Supported:")
    fmt.Println("  YouTube, YouTube Music, SoundCloud, Bandcamp, and 1000+ more")
    fmt.Println()
    gray.Println("  radii5 --version")
}
