package main

import (
	"github.com/lazuli-inc/ninjacrawler"
	"github.com/lazuli-inc/ninjacrawler/examples/concurrent/handlers/aqua"
	"github.com/lazuli-inc/ninjacrawler/examples/concurrent/handlers/kyocera"
	"github.com/lazuli-inc/ninjacrawler/examples/concurrent/handlers/markt"
	"github.com/lazuli-inc/ninjacrawler/examples/concurrent/handlers/sandvik"
)

func main() {
	ninjacrawler.NewNinjaCrawler().
		AddSite(kyocera.Crawler()).
		AddSite(aqua.Crawler()).
		AddSite(markt.Crawler()).
		AddSite(sandvik.Crawler()).
		Start()
}
