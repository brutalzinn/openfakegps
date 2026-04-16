package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/openfakegps/openfakegps/backend/internal/api"
	"github.com/openfakegps/openfakegps/backend/internal/grpcserver"
	"github.com/openfakegps/openfakegps/backend/internal/orchestration"
	"github.com/openfakegps/openfakegps/backend/internal/simulation"
)

func main() {
	grpcPort := flag.Int("grpc-port", 50051, "gRPC server port")
	httpPort := flag.Int("http-port", 3000, "HTTP API server port")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("starting OpenFakeGPS backend")

	// Create core components.
	registry := orchestration.NewRegistry()

	// The location callback sends position updates over the gRPC stream
	// to the device assigned to the simulation.
	locationCB := func(simID, deviceID string, pos simulation.Position) {
		stream := registry.GetStream(deviceID)
		if stream == nil {
			return
		}
		if err := stream.SendLocationUpdate(simID, pos); err != nil {
			log.Printf("error sending location update to %s: %v", deviceID, err)
		}
	}

	engine := simulation.NewEngine(locationCB)
	orch := orchestration.NewOrchestrator(registry, engine)

	// gRPC server.
	grpcSrv := grpc.NewServer()
	grpcHandler := grpcserver.NewServer(orch, registry)
	grpcHandler.Register(grpcSrv)

	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpcPort))
	if err != nil {
		log.Fatalf("failed to listen on gRPC port %d: %v", *grpcPort, err)
	}

	// HTTP API server.
	apiServer := api.NewServer(orch, engine, registry)
	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", *httpPort),
		Handler:      apiServer.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start servers.
	errCh := make(chan error, 2)

	go func() {
		log.Printf("gRPC server listening on :%d", *grpcPort)
		errCh <- grpcSrv.Serve(grpcLis)
	}()

	go func() {
		log.Printf("HTTP API server listening on :%d", *httpPort)
		errCh <- httpSrv.ListenAndServe()
	}()

	// Graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("received signal %v, shutting down", sig)
	case err := <-errCh:
		log.Printf("server error: %v", err)
	}

	log.Println("initiating graceful shutdown")

	// Stop gRPC server gracefully.
	grpcSrv.GracefulStop()

	// Stop HTTP server with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("shutdown complete")
}
