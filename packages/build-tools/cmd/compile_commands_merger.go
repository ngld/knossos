package cmd

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/rotisserie/eris"
	"github.com/spf13/cobra"
)

type ccItem struct {
	Command   string `json:"command"`
	Directory string `json:"directory"`
	File      string `json:"file"`
}

var mergeCompileCommansCmd = &cobra.Command{
	Use:   "merge-compile-commands <output file> <input files...>",
	Short: "Merges several compile_commands.json files. Assumes that only absolute paths are used.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return eris.Errorf("Expected at least 2 arguments but got %d!", len(args))
		}

		output := make([]ccItem, 0)
		var chunk []ccItem
		for _, fpath := range args[1:] {
			data, err := ioutil.ReadFile(fpath)
			if err != nil {
				return eris.Wrapf(err, "failed to read %s", fpath)
			}

			err = json.Unmarshal(data, &chunk)
			if err != nil {
				return eris.Wrapf(err, "failed to decode %s", fpath)
			}

			output = append(output, chunk...)
		}

		// Tell clangd about /mingw64/include
		for idx, item := range output {
			parts := strings.SplitN(item.Command, " ", 2)
			if strings.HasSuffix(parts[0], "msys64\\mingw64\\bin\\gcc.exe") {
				msysPath := strings.TrimSuffix(parts[0], "bin\\gcc.exe")
				item.Command += " -I" + msysPath + "include"
				output[idx] = item
			} else if strings.HasSuffix(parts[0], "msys64\\mingw64\\bin\\g++.exe") {
				msysPath := strings.TrimSuffix(parts[0], "bin\\g++.exe")
				item.Command += " -I" + msysPath + "include"
				output[idx] = item
			}
		}

		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return eris.Wrap(err, "failed to encode output")
		}

		err = ioutil.WriteFile(args[0], data, 0o660)
		if err != nil {
			return eris.Wrapf(err, "failed to write to %s", args[0])
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(mergeCompileCommansCmd)
}
