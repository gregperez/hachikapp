package initComponent

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func Init() {
	//RunLoadProperties()
	RunPrimaryProcess()
}

func Run() {
	Init()
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGKILL)

	s := <-signalChan
	log.Println("Got signal: ", s)
	os.Exit(0)
}
