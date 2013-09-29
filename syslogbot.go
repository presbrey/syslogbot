package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/presbrey/go/syslogbot/lib"
	"github.com/ziutek/syslog"
)

func main() {
	srv := syslogbot.NewBot(os.Args[1])
	s := syslog.NewServer()
	s.AddHandler(srv.SyslogHandler())
	s.Listen("0.0.0.0:514")
	s.Listen("0.0.0.0:1514")
	sc := make(chan os.Signal, 2)
	signal.Notify(sc, syscall.SIGTERM, syscall.SIGINT)
	<-sc
	log.Println("Shutdown the server...")
	s.Shutdown()
	log.Println("Server is down")
}
