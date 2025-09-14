package main

import (
	"fmt"
	"github.com/nsmithuk/resolver"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Создаем конфигурацию с включенным DNSSEC
	config := &resolver.Config{
		EnableDNSSEC: true,
		EnableCache:  true,
	}

	// Создаем и запускаем DNS сервер
	server := resolver.NewServerWithConfig(config)

	// Обработка сигналов для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем сервер в отдельной горутине
	go func() {
		if err := server.Start(); err != nil {
			fmt.Printf("Error starting DNS server: %v\n", err)
			os.Exit(1)
		}
	}()

	fmt.Println("DNS server is running on port 5355 with DNSSEC enabled")
	fmt.Println("Press Ctrl+C to stop")

	// Ожидаем сигнал для завершения
	<-sigChan
	fmt.Println("\nShutting down...")
}