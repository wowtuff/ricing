package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/wowtuff/ricing/api"
	"github.com/wowtuff/ricing/tools/toolset"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:1777", "address to listen on")
	uiDir := flag.String("ui-dir", "", "directory to serve UI from")
	flag.Parse()

	uiPath := ""
	if *uiDir != "" {
		uiPath, _ = filepath.Abs(*uiDir)
	}

	reg := toolset.NewDefaultRegistry()

	srv := api.NewServer(*addr, reg, uiPath)
	log.Printf("ricingd listening on http://%s\n", *addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
