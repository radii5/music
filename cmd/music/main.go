package main

import (
	"os"

	"github.com/radii5/music/cmd"
	"github.com/urfave/cli/v2"
)

var version = "dev"

func main() {
	app := &cli.App{
		Name:    "radii5",
		Usage:   "CLI music downloader powered by yt-dlp",
		Version: version,
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
				Value:   0, // 0 = adaptive based on file size
				Usage:   "Number of parallel download threads (0 = adaptive)",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return cli.ShowAppHelp(c)
			}

			url := c.Args().First()
			format := c.String("format")
			output := c.String("output")
			threads := c.Int("threads")

			cmd.RunWithOptions(url, format, output, threads)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}
