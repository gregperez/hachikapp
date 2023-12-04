package initComponent

import (
	"log"
	"reflect"
)

type IPrimaryProcess interface {
	Start()
}

var process = make([]IPrimaryProcess, 0)

func RegisterPrimaryProcess(p IPrimaryProcess) {
	process = append(process, p)
}

func RunPrimaryProcess() {
	for _, p := range process {
		log.Println("Start process: ", reflect.TypeOf(p).Elem().Name())
		go p.Start()
	}
}
