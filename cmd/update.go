package cmd

import (
	"fmt"
	"os"

	"github.com/dimitri/regresql/regresql"
	"github.com/spf13/cobra"
)

var versionedAll bool

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update [flags] [file ...]",
	Short: "Creates or updates the expected output files",
	Long: `Creates or updates the expected output files for all SQL queries.

When one or more SQL file paths are given as arguments, only those files
produce version-specific expected output (e.g. query.pg16.out); all other
files are updated with generic output as usual.

Use --versioned-all to write version-specific expected files for every query.
--versioned-all and file arguments are mutually exclusive.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := checkDirectory(cwd); err != nil {
			fmt.Printf(err.Error())
			os.Exit(1)
		}
		if versionedAll && len(args) > 0 {
			fmt.Println("Error: --versioned-all and file arguments are mutually exclusive")
			os.Exit(1)
		}
		regresql.Update(cwd, args, versionedAll)
	},
}

func init() {
	RootCmd.AddCommand(updateCmd)

	updateCmd.Flags().StringVarP(&cwd, "cwd", "C", ".", "Change to Directory")
	updateCmd.Flags().BoolVar(&versionedAll, "versioned-all", false,
		"Write version-specific expected files for all queries (e.g. query.pg16.out)")
}
