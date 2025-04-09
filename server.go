package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Server struct {
	httpAddr  string
	httpsAddr string
}

func NewServer(httpAddr, httpsAddr string) *Server {
	return &Server{httpAddr: httpAddr, httpsAddr: httpsAddr}
}

func (s *Server) Run(ctx context.Context) error {
	signalChan := make(chan os.Signal, 1)
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, os.Kill, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create an errgroup for managing multiple goroutines
	g, gctx := errgroup.WithContext(ctx)

	// Start two web services in separate goroutines
	g.Go(func() error { return httpServer(gctx, s.httpAddr) })
	g.Go(func() error { return httpsServer(gctx, s.httpsAddr) })

	// Listen for OS interrupts and cancel context
	go func() {
		<-signalChan // Block until an OS signal is received
		fmt.Println("Received interrupt signal, shutting down...")
		cancel() // Cancel the context
	}()

	// Wait for all goroutines to exit
	if err := g.Wait(); err != nil {
		fmt.Println("Error:", err)
		return err
	}

	fmt.Println("All services stopped. Exiting.")
	return nil
}

func httpServer(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", httpHandler)
	mux.HandleFunc("GET /error", errorHandler)
	mux.Handle("GET /.well-known/acme-challenge/", http.StripPrefix("/.well-known/acme-challenge/", http.FileServer(http.Dir("/challenge/.well-known/acme-challenge/"))))

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	errChan := make(chan error, 1)
	defer close(errChan)

	go func() {
		fmt.Println("Starting HTTP server on", addr)
		// Return ListenAndServe error directly so errgroup can handle it
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("Shutting down HTTP server on", addr)
		httpServer.SetKeepAlivesEnabled(false)
		return httpServer.Shutdown(ctx) // Gracefully shutdown server
	case err := <-errChan:
		return err
	}
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprintf(w, "Hello World %s", generateRandomString(10))
}

func generateRandomString(n int) string {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))

		ret[i] = letters[num.Int64()]
	}
	return string(ret)
}

func httpsServer(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", httpHandler)
	mux.Handle("GET /.well-known/acme-challenge/", http.StripPrefix("/.well-known/acme-challenge/", http.FileServer(http.Dir("/challenge/.well-known/acme-challenge/"))))

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	errChan := make(chan error, 1)
	defer close(errChan)

	go func() {
		fmt.Println("Starting HTTPS server on", addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			//return fmt.Errorf("error starting HTTP server:%w", err)
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("Shutting down HTTPS server on", addr)
		httpServer.SetKeepAlivesEnabled(false)
		return httpServer.Shutdown(ctx) // Gracefully shutdown server
	case err := <-errChan:
		return err
	}
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	//log.Fatalf("crash")
	panic("simulated server crash")
}
