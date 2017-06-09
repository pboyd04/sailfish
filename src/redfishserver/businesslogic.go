package redfishserver

import (
	"bytes"
	"fmt"
	"math/rand"
)

func (rh *Config) Startup() (done chan struct{}) {
	// let's hook up some test data in service root for now to see how it would look
	j := rh.odata["/redfish/v1/"].(map[string]interface{})
	j["madness"] = slowdata{}

	done = make(chan struct{})
	return done
}

type slowdata struct{}

func (this slowdata) MarshalJSON() ([]byte, error) {
	outstr := fmt.Sprintf(`{"msg": "LETS GO CRAZY %d TIMES"}`, rand.Uint32())
	buffer := bytes.NewBufferString(outstr)
	return buffer.Bytes(), nil
}