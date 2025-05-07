package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v3"
	"sql.garden/schemas"
)

func main() {
	cmd := &cli.Command{
		Name:  "sqlgarden",
		Usage: "",
		Action: func(context.Context, *cli.Command) error {
			fmt.Println("sow thy seed")
			return nil
		},
		Commands: []*cli.Command{
			{
				Name: "install",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "schema", Required: true},
					&cli.StringFlag{Name: "engine", Value: "sqlite"},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					if !schemas.IsSupported(c.String("engine")) {
						return errors.New("engine not supported")
					}

					return nil
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
