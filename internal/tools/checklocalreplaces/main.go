// Command checklocalreplaces rejects filesystem replacements from go.mod data.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type moduleRef struct {
	Path    string
	Version string
}

type replacement struct {
	Old moduleRef
	New moduleRef
}

type moduleFile struct {
	Replace []replacement
}

func main() {
	var mod moduleFile
	if err := json.NewDecoder(os.Stdin).Decode(&mod); err != nil {
		fmt.Fprintf(os.Stderr, "decode go.mod JSON: %v\n", err)
		os.Exit(2)
	}

	failed := false
	for _, replace := range mod.Replace {
		if replace.New.Version != "" {
			continue
		}
		fmt.Fprintf(
			os.Stderr,
			"local module replacement is not allowed: %s => %s\n",
			replace.Old.Path,
			replace.New.Path,
		)
		failed = true
	}

	if failed {
		os.Exit(1)
	}
}
