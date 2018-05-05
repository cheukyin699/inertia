package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/ubclaunchpad/inertia/common"
)

// Client manages a deployment
type Client struct {
	*RemoteVPS
	version   string
	project   string
	buildType string
}

// NewClient sets up a client to communicate to the daemon at
// the given named remote.
func NewClient(remoteName string, config *Config) (*Client, bool) {
	remote, found := config.GetRemote(remoteName)
	if !found {
		return nil, false
	}

	return &Client{
		RemoteVPS: remote,
	}, false
}

// BootstrapRemote configures a remote vps for continuous deployment
// by installing docker, starting the daemon and building a
// public-private key-pair. It outputs configuration information
// for the user.
func (c *Client) BootstrapRemote(runner SSHSession) error {
	println("Setting up remote \"" + c.Name + "\" at " + c.IP)

	println(">> Step 1/4: Installing docker...")
	err := c.installDocker(runner)
	if err != nil {
		return err
	}

	println("\n>> Step 2/4: Building deploy key...")
	if err != nil {
		return err
	}
	pub, err := c.keyGen(runner)
	if err != nil {
		return err
	}

	// This step needs to run before any other commands that rely on
	// the daemon image, since the daemon is loaded here.
	println("\n>> Step 3/4: Starting daemon...")
	if err != nil {
		return err
	}
	err = c.DaemonUp(runner, c.version, c.IP, c.Daemon.Port)
	if err != nil {
		return err
	}

	println("\n>> Step 4/4: Fetching daemon API token...")
	token, err := c.getDaemonAPIToken(runner, c.version)
	if err != nil {
		return err
	}
	c.Daemon.Token = token

	println("\nInertia has been set up and daemon is running on remote!")
	println("You may have to wait briefly for Inertia to set up some dependencies.")
	fmt.Printf("Use 'inertia %s logs --stream' to check on the daemon's setup progress.\n\n", c.Name)

	println("=============================\n")

	// Output deploy key to user.
	println(">> GitHub Deploy Key (add to https://www.github.com/<your_repo>/settings/keys/new): ")
	println(pub.String())

	// Output Webhook url to user.
	println(">> GitHub WebHook URL (add to https://www.github.com/<your_repo>/settings/hooks/new): ")
	println("WebHook Address:  https://" + c.IP + ":" + c.Daemon.Port + "/webhook")
	println("WebHook Secret:   " + c.Daemon.Secret)
	println(`Note that you will have to disable SSH verification in your webhook
settings - Inertia uses self-signed certificates that GitHub won't
be able to verify.` + "\n")

	println(`Inertia daemon successfully deployed! Add your webhook url and deploy
key to enable continuous deployment.`)
	fmt.Printf("Then run 'inertia %s up' to deploy your application.\n", c.Name)

	return nil
}

// DaemonUp brings the daemon up on the remote instance.
func (c *Client) DaemonUp(session SSHSession, daemonVersion, host, daemonPort string) error {
	scriptBytes, err := Asset("client/bootstrap/daemon-up.sh")
	if err != nil {
		return err
	}

	// Run inertia daemon.
	daemonCmdStr := fmt.Sprintf(string(scriptBytes), daemonVersion, daemonPort, host)
	return session.RunStream(daemonCmdStr, false)
}

// DaemonDown brings the daemon down on the remote instance
func (c *Client) DaemonDown(session SSHSession) error {
	scriptBytes, err := Asset("client/bootstrap/daemon-down.sh")
	if err != nil {
		return err
	}

	_, stderr, err := session.Run(string(scriptBytes))
	if err != nil {
		println(stderr.String())
		return err
	}

	return nil
}

// Up brings the project up on the remote VPS instance specified
// in the deployment object.
func (c *Client) Up(gitRemote, buildType string, stream bool) (*http.Response, error) {
	if buildType == "" {
		buildType = c.buildType
	}

	reqContent := &common.DaemonRequest{
		Stream:    stream,
		Project:   c.project,
		BuildType: buildType,
		Secret:    c.Daemon.Secret,
		GitOptions: &common.GitOptions{
			RemoteURL: common.GetSSHRemoteURL(gitRemote),
			Branch:    c.Branch,
		},
	}
	return c.request("POST", "/up", reqContent)
}

// Down brings the project down on the remote VPS instance specified
// in the configuration object.
func (c *Client) Down() (*http.Response, error) {
	return c.request("POST", "/down", nil)
}

// Status lists the currently active containers on the remote VPS instance
func (c *Client) Status() (*http.Response, error) {
	resp, err := c.request("GET", "/status", nil)
	if err != nil &&
		(strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "refused")) {
		return nil, fmt.Errorf("daemon on remote %s appears offline or inaccessible", c.Name)
	}
	return resp, err
}

// Reset shuts down deployment and deletes the contents of the deployment's
// project directory
func (c *Client) Reset() (*http.Response, error) {
	return c.request("POST", "/reset", nil)
}

// Logs get logs of given container
func (c *Client) Logs(stream bool, container string) (*http.Response, error) {
	reqContent := &common.DaemonRequest{
		Stream:    stream,
		Container: container,
	}
	return c.request("GET", "/logs", reqContent)
}

// AddUser adds an authorized user for access to Inertia Web
func (c *Client) AddUser(username, password string, admin bool) (*http.Response, error) {
	reqContent := &common.UserRequest{
		Username: username,
		Password: password,
		Admin:    admin,
	}
	return c.request("POST", "/user/adduser", reqContent)
}

// RemoveUser prevents a user from accessing Inertia Web
func (c *Client) RemoveUser(username string) (*http.Response, error) {
	reqContent := &common.UserRequest{Username: username}
	return c.request("POST", "/user/removeuser", reqContent)
}

// ResetUsers resets all users on the remote.
func (c *Client) ResetUsers() (*http.Response, error) {
	return c.request("POST", "/user/resetusers", nil)
}

// ListUsers lists all users on the remote.
func (c *Client) ListUsers() (*http.Response, error) {
	return c.request("GET", "/user/listusers", nil)
}

func (c *Client) request(method, endpoint string, requestBody interface{}) (*http.Response, error) {
	// Assemble URL
	url, err := url.Parse("https://" + c.RemoteVPS.GetIPAndPort())
	if err != nil {
		return nil, err
	}
	url.Path = path.Join(url.Path, endpoint)
	urlString := url.String()

	// Assemble request
	var payload io.Reader
	if requestBody != nil {
		body, err := json.Marshal(requestBody)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(body)
	} else {
		payload = nil
	}
	req, err := http.NewRequest(method, urlString, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Daemon.Token)

	// Make HTTPS request
	tr := &http.Transport{
		// Our certificates are self-signed, so will raise
		// a warning - currently, we ask our client to ignore
		// this warning.
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{Transport: tr}
	return client.Do(req)
}
