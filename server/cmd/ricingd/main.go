package main

import (
	"flag"
	"log"

	"github.com/wowtuff/ricing/api"
	"github.com/wowtuff/ricing/tools/toolset"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:1777", "address to listen on")
	flag.Parse()

	reg := toolset.NewDefaultRegistry()

	srv := api.NewServer(*addr, reg)
	log.Printf("ricingd listening on http://%s\n", *addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
