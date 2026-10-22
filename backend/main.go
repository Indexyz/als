package main

import (
	"flag"

	"github.com/samlm0/als/v2/als"
	"github.com/samlm0/als/v2/config"
)

func main() {
	flag.Parse()
	config.LoadWebConfig()
	als.Init()
}
