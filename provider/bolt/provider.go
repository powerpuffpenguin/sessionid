package bolt

import (
	"fmt"
	"log"

	"github.com/boltdb/bolt"
	"github.com/powerpuffpenguin/sessionid"
)

func Open(filename string) {
	db, e := bolt.Open(filename, 0600, nil)
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	fmt.Println(`success`, sessionid.NewManager())

}
