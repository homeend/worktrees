package main

import (
	"os"

	cmd "github.com/code-drill/wt/cmd/wt"
)

func main() {
	os.Exit(cmd.Execute())
}
