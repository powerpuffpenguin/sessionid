package configure

import (
	"encoding/json"
	"fmt"

	"github.com/google/go-jsonnet"
)

var defaultConfigure Configure

func DefaultConfigure() *Configure {
	return &defaultConfigure
}

type Configure struct {
}

func Load(filename string) (e error) {
	vm := jsonnet.MakeVM()
	vm.Importer(&jsonnet.FileImporter{})
	jsonStr, e := vm.EvaluateFile(filename)
	if e != nil {
		return
	}
	cnf := DefaultConfigure()
	fmt.Println(jsonStr)
	e = json.Unmarshal([]byte(jsonStr), cnf)
	return
}
