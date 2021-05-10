package main

import (
	"flag"
	"log"

	"github.com/powerpuffpenguin/sessionid/server/configure"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var (
		help bool
		cnf  string
	)
	flag.BoolVar(&help, `help`, false, `display help`)
	flag.StringVar(&cnf, `conf`, `server.jsonnet`, `configure file`)

	flag.Parse()
	if help {
		flag.PrintDefaults()
	} else {
		e := configure.Load(cnf)
		if e != nil {
			log.Fatalln(e)
		}
	}
}
