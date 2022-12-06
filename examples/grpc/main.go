package main

import (
	"fmt"
	"github.com/wind-c/comqtt/plugin/auth/grpc"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/logrusorgru/aurora"

	mqtt "github.com/wind-c/comqtt/server"
	"github.com/wind-c/comqtt/server/listeners"
)

func main() {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	fmt.Println(aurora.Magenta("CoMQTT Server initializing..."), aurora.Cyan("TCP"))

	// An example of configuring various server options...
	options := &mqtt.Options{
		BufferSize:      0, // Use default values
		BufferBlockSize: 0, // Use default values
	}

	server := mqtt.NewServer(options)
	tcp := listeners.NewTCP("t1", ":1883")

	auth, err := grpc.New("./plugin/auth/grpc/conf.yml")
	if err != nil {
		log.Fatal(err)
	}
	err = auth.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer auth.Close()

	err = server.AddListener(tcp, &listeners.Config{
		Auth: auth,
	})
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()
	fmt.Println(aurora.BgMagenta("  Started!  "))

	<-done
	fmt.Println(aurora.BgRed("  Caught Signal  "))

	server.Close()
	fmt.Println(aurora.BgGreen("  Finished  "))
}
