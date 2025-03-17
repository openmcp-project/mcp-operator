package main

import (
	"context"
	"fmt"
	"os"

	"github.tools.sap/CoLa/mcp-operator/cmd/mcp-operator/app"
)

func main() {
	ctx := context.Background()
	defer ctx.Done()
	cmd := app.NewMCPOperatorCommand(ctx)

	if err := cmd.Execute(); err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
}
