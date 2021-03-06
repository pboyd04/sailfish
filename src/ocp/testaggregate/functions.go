package testaggregate

import (
	"sync"

	"github.com/Knetic/govaluate"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
)

var functions map[string]govaluate.ExpressionFunction
var functionsMu *sync.RWMutex

func init() {
	// share our functions with awesome mapper
	functions, functionsMu = awesome_mapper2.InitFunctions()

	awesome_mapper2.AddFunction("add_attribute_property",
		func(args ...interface{}) (interface{}, error) {
			return map[string]map[string]map[string]interface{}{}, nil
		})
}
