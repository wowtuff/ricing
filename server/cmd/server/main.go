package main

import (
	"context"
	"fmt"
	"log"

	"github.com/wowtuff/ricing/agent"
	"github.com/wowtuff/ricing/tools/toolset"
)

func main() {
	reg := toolset.NewDefaultRegistry()

	answer, err := agent.Run(
		context.Background(),
		reg,
		"What is 6 times 7? Then notify user.",
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(answer)
}
