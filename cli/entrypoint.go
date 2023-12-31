package cli

import (
	"fmt"

	"github.com/carlmontanari/slurpeeth/slurpeeth"
	"github.com/urfave/cli/v2"
)

const (
	configFlag = "config"
	reloadFlag = "reload"
	debugFlag  = "debug"
)

// ShowVersion shows the clabernetes version information for clabernetes CLI tools.
func ShowVersion(_ *cli.Context) {
	fmt.Printf("\tversion: %s\n", slurpeeth.Version)                            //nolint:forbidigo
	fmt.Printf("\tsource : %s\n", "https://github.com/carlmontanari/slurpeeth") //nolint:forbidigo
}

// Entrypoint loads the slurpeeth config, creates the slurpeeth process and starts it.
func Entrypoint() *cli.App {
	cli.VersionPrinter = ShowVersion

	return &cli.App{
		Name:    "slurpeeth",
		Version: slurpeeth.Version,
		Usage:   "run slurpeeth!",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     configFlag,
				Usage:    "slurpeeth configuration file to load",
				Required: false,
				Value:    "slurpeeth.yaml",
			},
			&cli.BoolFlag{
				Name:     reloadFlag,
				Usage:    "live reload slurpeeth configuration file",
				Required: false,
				Value:    false,
			},
			&cli.BoolFlag{
				Name:     debugFlag,
				Usage:    "enable debug logging",
				Required: false,
				Value:    false,
			},
		},
		Action: func(ctx *cli.Context) error {
			m, err := slurpeeth.GetManager(
				slurpeeth.WithConfigFile(ctx.String(configFlag)),
				slurpeeth.WithLiveReload(ctx.Bool(reloadFlag)),
				slurpeeth.WithDebug(ctx.Bool(debugFlag)),
			)
			if err != nil {
				return err
			}

			return m.Run()
		},
	}
}
