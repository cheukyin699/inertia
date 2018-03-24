package main

import (
	"context"
	"net/http"
	"os"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/ubclaunchpad/inertia/daemon/inertia/auth"
	"github.com/ubclaunchpad/inertia/daemon/inertia/project"
)

// daemonVersion indicates the daemon's corresponding Inertia daemonVersion
var daemonVersion string

const (
	// specify location of SSL certificate
	sslDirectory  = "/app/host/ssl/"
	daemonSSLCert = sslDirectory + "daemon.cert"
	daemonSSLKey  = sslDirectory + "daemon.key"
)

// run starts the daemon
func run(host, port, version string) {
	daemonVersion = version

	// Download docker-compose image
	println("Downloading docker-compose...")
	cli, err := docker.NewEnvClient()
	if err != nil {
		println(err.Error())
		println("Failed to start Docker client - shutting down daemon.")
		return
	}
	_, err = cli.ImagePull(context.Background(), project.DockerComposeVersion, types.ImagePullOptions{})
	if err != nil {
		println(err.Error())
		println("Failed to pull docker-compose image - shutting down daemon.")
		cli.Close()
		return
	}
	cli.Close()

	// Check if the cert files are available.
	println("Checking for existing SSL certificates in " + sslDirectory + "...")
	_, err = os.Stat(daemonSSLCert)
	certNotPresent := os.IsNotExist(err)
	_, err = os.Stat(daemonSSLKey)
	keyNotPresent := os.IsNotExist(err)
	sslRequirementsPresent := !(keyNotPresent && certNotPresent)

	// If they are not available, generate new ones.
	if !sslRequirementsPresent {
		println("No certificates found - generating new ones...")
		err = auth.GenerateCertificate(daemonSSLCert, daemonSSLKey, host+":"+port, "RSA")
		if err != nil {
			println(err.Error())
			return
		}
	}

	// API endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/up", auth.Authorized(upHandler, auth.GetAPIPrivateKey))
	mux.HandleFunc("/down", auth.Authorized(downHandler, auth.GetAPIPrivateKey))
	mux.HandleFunc("/status", auth.Authorized(statusHandler, auth.GetAPIPrivateKey))
	mux.HandleFunc("/reset", auth.Authorized(resetHandler, auth.GetAPIPrivateKey))
	mux.HandleFunc("/logs", auth.Authorized(logHandler, auth.GetAPIPrivateKey))
	mux.HandleFunc("/health-check", auth.Authorized(auth.HealthCheckHandler, auth.GetAPIPrivateKey))

	// GitHub webhook endpoint
	mux.HandleFunc("/", gitHubWebHookHandler)

	// Inertia web
	mux.Handle("/admin/", http.StripPrefix("/admin/", http.FileServer(http.Dir("/app/inertia-web"))))

	// Serve daemon on port
	println("Serving daemon on port " + port)
	println(http.ListenAndServeTLS(
		":"+port,
		daemonSSLCert,
		daemonSSLKey,
		mux,
	))
}
