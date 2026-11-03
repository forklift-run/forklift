package plt

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/forklift-run/forklift/internal/app/forklift"
	fcli "github.com/forklift-run/forklift/internal/app/forklift/cli"
)

// ls-file

func lsFileAction(ctx context.Context, c *cli.Command) error {
	plt, _, err := processFullBaseArgs(ctx, c, processingOptions{
		enableOverrides: true,
		mergePallet:     true,
	})
	if err != nil {
		return err
	}

	filter := c.Args().First()
	if filter == "" {
		// Exclude hidden directories such as `.git`
		filter = "{*,[^.]*/**}"
	}
	paths, err := forklift.ListPalletFiles(plt, filter)
	if err != nil {
		return err
	}
	for _, p := range paths {
		fmt.Println(p)
	}
	return nil
}

// locate-file

func locateFileAction(ctx context.Context, c *cli.Command) error {
	plt, _, err := processFullBaseArgs(ctx, c, processingOptions{
		enableOverrides: true,
		mergePallet:     true,
	})
	if err != nil {
		return err
	}

	location, err := forklift.GetFileLocation(plt, c.Args().First())
	if err != nil {
		return err
	}
	fmt.Println(location)
	return nil
}

// show-file

func showFileAction(ctx context.Context, c *cli.Command) error {
	plt, _, err := processFullBaseArgs(ctx, c, processingOptions{
		enableOverrides: true,
		mergePallet:     true,
	})
	if err != nil {
		return err
	}

	return fcli.FprintFile(os.Stdout, plt, c.Args().First())
}

// edit-file

func editFileAction(ctx context.Context, c *cli.Command) error {
	plt, _, err := processFullBaseArgs(ctx, c, processingOptions{
		enableOverrides: true,
		mergePallet:     true,
	})
	if err != nil {
		return err
	}

	return fcli.EditFileWithCOW(ctx, plt, c.Args().First(), c.String("editor"))
}

// del-file

func delFileAction(ctx context.Context, c *cli.Command) error {
	plt, _, err := processFullBaseArgs(ctx, c, processingOptions{
		enableOverrides: true,
		mergePallet:     true,
	})
	if err != nil {
		return err
	}

	return fcli.RemoveFile(0, plt, c.Args().First())
}
