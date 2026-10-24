package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"smarthome/internal/config"
	"smarthome/internal/gateway"
	"smarthome/internal/server"
)

func main() {
	defaultConfig := config.Default()

	portFlag := flag.Int("port", defaultConfig.ListenPort, "port to listen on")
	gatewayFlag := flag.String("gateway", defaultConfig.GatewayURL, "gateway base url")
	tokenFlag := flag.String("token", defaultConfig.Token, "gateway bearer token")
	flag.Parse()

	if *tokenFlag == "" {
		fmt.Fprintln(os.Stderr, "error: gateway token is required")
		os.Exit(1)
	}

	gatewayClient := gateway.NewClient(*gatewayFlag, *tokenFlag)
	commandArgs := flag.Args()

	if len(commandArgs) == 0 || commandArgs[0] == "serve" {
		runServer(*portFlag, gatewayClient)
		return
	}

	executeCLI(commandArgs, gatewayClient)
}

func runServer(listenPort int, gatewayClient *gateway.Client) {
	serviceContext, cancelFunction := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancelFunction()

	initialSyncContext, initialSyncCancel := context.WithTimeout(
		serviceContext,
		5*time.Second,
	)
	_ = gatewayClient.Sync(initialSyncContext)
	initialSyncCancel()

	gatewayClient.StartBackgroundSync(serviceContext, 5*time.Second)

	listenAddress := fmt.Sprintf(":%d", listenPort)
	serverInstance := server.NewServer(listenAddress, gatewayClient)

	go func() {
		<-serviceContext.Done()
		_ = serverInstance.Close()
	}()

	fmt.Printf("smarthome backend listening on http://127.0.0.1%s\n", listenAddress)
	if errorServe := serverInstance.Start(); errorServe != nil {
		fmt.Fprintf(os.Stderr, "server stopped: %v\n", errorServe)
	}
}

func executeCLI(commandArgs []string, gatewayClient *gateway.Client) {
	commandContext, cancelCommand := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancelCommand()

	_ = gatewayClient.Sync(commandContext)

	subCommand := commandArgs[0]
	switch subCommand {
	case "status":
		deviceList := gatewayClient.Devices()
		formattedBytes, _ := json.MarshalIndent(deviceList, "", "  ")
		fmt.Println(string(formattedBytes))

	case "get":
		if len(commandArgs) < 2 {
			fmt.Fprintln(os.Stderr, "usage: smarthome get <device_id>")
			os.Exit(1)
		}
		targetDevice, exists := gatewayClient.Device(commandArgs[1])
		if !exists {
			fmt.Fprintln(os.Stderr, "device not found")
			os.Exit(1)
		}
		formattedBytes, _ := json.MarshalIndent(targetDevice, "", "  ")
		fmt.Println(string(formattedBytes))

	case "set":
		if len(commandArgs) < 3 {
			fmt.Fprintln(os.Stderr, "usage: smarthome set <device_id> <on|off>")
			os.Exit(1)
		}
		if errorSet := gatewayClient.SetState(
			commandContext,
			commandArgs[1],
			commandArgs[2],
		); errorSet != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", errorSet)
			os.Exit(1)
		}
		fmt.Println("ok")

	case "toggle":
		if len(commandArgs) < 2 {
			fmt.Fprintln(os.Stderr, "usage: smarthome toggle <device_id>")
			os.Exit(1)
		}
		if errorToggle := gatewayClient.Toggle(
			commandContext,
			commandArgs[1],
		); errorToggle != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", errorToggle)
			os.Exit(1)
		}
		fmt.Println("ok")

	case "sync":
		if errorSync := gatewayClient.Sync(commandContext); errorSync != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", errorSync)
			os.Exit(1)
		}
		fmt.Println("synchronized")

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", subCommand)
		os.Exit(1)
	}
}
