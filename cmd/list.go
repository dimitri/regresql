// Copyright © 2017 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"os"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/dimitri/regresql/regresql"
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

		stat, err := os.Stat(cwd)
		if err != nil {
			panic(err)
		}
		if ! stat.IsDir() {
			panic(fmt.Sprintf("%s is not a directory!", cwd))
		}
		regresql.List(cwd)
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&cwd, "cwd", "C", ".", "Change to Directory")
}
