package main

import (
	"fmt"
	"os"

	"github.com/radii5/music/cmd"
	"github.com/urfave/cli/v2"
)

var version = "dev"

func main() {
	app := &cli.App{
		Name:                   "radii5",
		Usage:                  "CLI music downloader powered by yt-dlp",
		Version:                version,
		UseShortOptionHandling: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "mp3",
				Usage:   "Audio format (mp3, flac, wav, m4a, opus)",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Value:   "~/Music/radii5 downloads",
				Usage:   "Output directory",
			},
			&cli.IntFlag{
				Name:    "threads",
				Aliases: []string{"t"},
				Value:   0,
				Usage:   "Number of parallel download threads (0 = adaptive)",
			},
			&cli.IntFlag{
				Name:    "workers",
				Aliases: []string{"w"},
				Value:   4,
				Usage:   "Number of concurrent download workers for playlists",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return cli.ShowAppHelp(c)
			}

			// Build arguments array for cmd.Run
			args := []string{}

			// Add format flag if specified
			if format := c.String("format"); format != "mp3" {
				args = append(args, "--format", format)
			}

			// Add output flag if specified
			if output := c.String("output"); output != "~/Music/radii5 downloads" {
				args = append(args, "--output", output)
			}

			// Add threads flag if specified
			if threads := c.Int("threads"); threads != 0 {
				args = append(args, "--threads", fmt.Sprintf("%d", threads))
			}

			// Add workers flag if specified
			if workers := c.Int("workers"); workers != 4 {
				args = append(args, "--workers", fmt.Sprintf("%d", workers))
			}

			// Add URL
			args = append(args, c.Args().First())

			cmd.Run(args)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}
