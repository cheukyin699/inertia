package client

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func getTestConfig(writer io.Writer) *Config {
	config := &Config{
		Writer:  writer,
		Version: "test",
	}
	return config
}

func getInstrumentedTestRemote() *RemoteVPS {
	remote := &RemoteVPS{
		IP:   "0.0.0.0",
		PEM:  "../test_env/test_key",
		User: "root",
		Daemon: &DaemonConfig{
			Port: "8081",
		},
	}
	travis := os.Getenv("TRAVIS")
	if travis != "" {
		remote.Daemon.SSHPort = "69"
	} else {
		remote.Daemon.SSHPort = "22"
	}
	return remote
}

func TestInstallDocker(t *testing.T) {
	remote := getInstrumentedTestRemote()
	script, err := ioutil.ReadFile("bootstrap/docker.sh")
	assert.Nil(t, err)

	// Make sure the right command is run.
	session := mockSSHRunner{r: remote}
	remote.installDocker(&session)
	assert.Equal(t, string(script), session.Calls[0])
}

func TestDaemonUp(t *testing.T) {
	remote := getInstrumentedTestRemote()
	script, err := ioutil.ReadFile("bootstrap/daemon-up.sh")
	assert.Nil(t, err)
	actualCommand := fmt.Sprintf(string(script), "latest", "8081", "0.0.0.0")

	// Make sure the right command is run.
	session := mockSSHRunner{r: remote}

	// Make sure the right command is run.
	err = remote.DaemonUp(&session, "latest", "0.0.0.0", "8081")
	assert.Nil(t, err)
	println(actualCommand)
	assert.Equal(t, actualCommand, session.Calls[0])
}

func TestKeyGen(t *testing.T) {
	remote := getInstrumentedTestRemote()
	script, err := ioutil.ReadFile("bootstrap/token.sh")
	assert.Nil(t, err)
	tokenScript := fmt.Sprintf(string(script), "test")

	// Make sure the right command is run.
	session := mockSSHRunner{r: remote}

	// Make sure the right command is run.
	_, err = remote.getDaemonAPIToken(&session, "test")
	assert.Nil(t, err)
	assert.Equal(t, session.Calls[0], tokenScript)
}

func TestBootstrap(t *testing.T) {
	remote := getInstrumentedTestRemote()
	dockerScript, err := ioutil.ReadFile("bootstrap/docker.sh")
	assert.Nil(t, err)

	keyScript, err := ioutil.ReadFile("bootstrap/keygen.sh")
	assert.Nil(t, err)

	script, err := ioutil.ReadFile("bootstrap/token.sh")
	assert.Nil(t, err)
	tokenScript := fmt.Sprintf(string(script), "test")

	script, err = ioutil.ReadFile("bootstrap/daemon-up.sh")
	assert.Nil(t, err)
	daemonScript := fmt.Sprintf(string(script), "test", "8081", "0.0.0.0")

	var writer bytes.Buffer
	session := mockSSHRunner{r: remote}
	err = remote.Bootstrap(&session, "gcloud", getTestConfig(&writer))
	assert.Nil(t, err)

	// Make sure all commands are formatted correctly
	assert.Equal(t, string(dockerScript), session.Calls[0])
	assert.Equal(t, string(keyScript), session.Calls[1])
	assert.Equal(t, daemonScript, session.Calls[2])
	assert.Equal(t, tokenScript, session.Calls[3])
}

func TestInstrumentedBootstrap(t *testing.T) {
	remote := getInstrumentedTestRemote()
	session := &SSHRunner{r: remote}
	var writer bytes.Buffer
	err := remote.Bootstrap(session, "testvps", getTestConfig(&writer))
	assert.Nil(t, err)

	// Daemon setup takes a bit of time - do a crude wait
	time.Sleep(3 * time.Second)

	// Check if daemon is online following bootstrap
	host := "https://" + remote.GetIPAndPort()
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get(host)
	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	assert.Nil(t, err)
}