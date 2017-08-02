package cmd

import (
	"fmt"

	"github.com/dimitri/regresql/regresql"
	"github.com/spf13/cobra"
)

// Command Flags
var (
	cwd string
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list candidates SQL files",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("list called on %s\n", cwd)

		checkDirectory(cwd)
		regresql.List(cwd)
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&cwd, "cwd", "C", ".", "Change to Directory")
}
