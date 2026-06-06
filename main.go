package main

import (
	"os"

	cmd "github.com/homeend/worktrees/cmd/wt"
)

func main() {
	os.Exit(cmd.Execute())
}
