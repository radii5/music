package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
	
	"github.com/fatih/color"
	"github.com/radii5/radii5/cmd"
)

var version = "0.1.0"

// Windows QuickEdit mode constants
const (
	enableQuickEditMode = 0x0040
	enableExtendedFlags = 0x0080
)

// disableQuickEdit prevents Windows terminal from pausing on click
func disableQuickEdit() {
	if runtime.GOOS != "windows" {
		return
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getStdHandle := kernel32.NewProc("GetStdHandle")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	// STD_INPUT_HANDLE = -10
	handle, _, _ := getStdHandle.Call(uintptr(^uint(10) + 1))

	var mode uint32
	getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))

	// Disable QuickEdit, enable extended flags
	mode &^= enableQuickEditMode
	mode |= enableExtendedFlags

	setConsoleMode.Call(handle, uintptr(mode))
}

func main() {
	// Disable QuickEdit mode on Windows to prevent accidental pausing
	disableQuickEdit()

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