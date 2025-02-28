package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	kwilcrypto "github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/util"
)

// Constants for the test setup
const (
	networkName      = "truf-test-network"
	TestKwilProvider = "http://localhost:8484"
	TestPrivateKey   = "1111111111111111111111111111111111111111111111111111111111111111" // Test private key
)

// ServerFixture is a fixture for setting up and tearing down a Trufnetwork server for testing
type ServerFixture struct {
	t          *testing.T
	docker     *docker
	client     *tnclient.Client
	ctx        context.Context
	containers struct {
		postgres containerSpec
		tsndb    containerSpec
	}
}

// containerSpec defines the configuration for a container
type containerSpec struct {
	name        string
	image       string
	tmpfsPath   string
	envVars     []string
	healthCheck func(d *docker) error
}

// docker provides a simplified interface for docker operations
type docker struct {
	t *testing.T
}

// NewServerFixture creates a new server fixture
func NewServerFixture(t *testing.T) *ServerFixture {
	ctx := context.Background()
	d := newDocker(t)

	return &ServerFixture{
		t:      t,
		docker: d,
		ctx:    ctx,
		containers: struct {
			postgres containerSpec
			tsndb    containerSpec
		}{
			postgres: containerSpec{
				name:      "test-kwil-postgres",
				image:     "kwildb/postgres:latest",
				tmpfsPath: "/var/lib/postgresql/data",
				envVars:   []string{"POSTGRES_HOST_AUTH_METHOD=trust"},
				healthCheck: func(d *docker) error {
					_, err := d.exec("test-kwil-postgres", "pg_isready", "-U", "postgres")
					return err
				},
			},
			tsndb: containerSpec{
				name:      "test-tsn-db",
				image:     "tsn-db:local",
				tmpfsPath: "/root/.kwild",
				envVars: []string{
					"CONFIG_PATH=/root/.kwild",
					"KWILD_APP_HOSTNAME=test-tsn-db",
					"KWILD_APP_PG_DB_HOST=test-kwil-postgres",
					"KWILD_CHAIN_P2P_EXTERNAL_ADDRESS=http://test-tsn-db:26656",
				},
				healthCheck: func(d *docker) error {
					// Wait for the service to be ready
					time.Sleep(5 * time.Second)
					_, err := d.exec("test-tsn-db", "ps", "aux")
					return err
				},
			},
		},
	}
}

// Setup sets up the test environment
func (s *ServerFixture) Setup() error {
	// Clean up any existing resources
	s.docker.cleanup()

	// Create network
	if err := s.docker.setupNetwork(); err != nil {
		return err
	}

	// Start postgres first
	if err := s.docker.startContainer(s.containers.postgres); err != nil {
		return err
	}

	// Wait for postgres to be healthy
	for i := 0; i < 10; i++ {
		if err := s.containers.postgres.healthCheck(s.docker); err == nil {
			break
		}
		if i == 9 {
			return errors.New("postgres failed to become healthy")
		}
		time.Sleep(time.Second)
	}

	// Start tsn-db with autogen
	s.t.Log("Starting tsn-db container...")
	if err := s.docker.startContainer(s.containers.tsndb); err != nil {
		// Get logs before failing
		if out, err := s.docker.run("logs", s.containers.tsndb.name); err == nil {
			s.t.Logf("TSN-DB container logs:\n%s", out)
		} else {
			s.t.Logf("Failed to get TSN-DB logs: %v", err)
		}
		// Get container status
		if status, err := s.docker.run("inspect", "--format", "{{.State.Status}}", s.containers.tsndb.name); err == nil {
			s.t.Logf("TSN-DB container status: %s", status)
		}
		return fmt.Errorf("failed to start tsn-db container: %w", err)
	}
	s.t.Log("TSN-DB container started successfully")

	// Wait for node to be fully initialized
	s.t.Log("Waiting for node to be fully initialized...")
	for i := 0; i < 30; i++ { // 30 seconds max wait
		healthCmd := exec.Command("curl", "-s", TestKwilProvider+"/api/v1/health")
		healthOut, healthErr := healthCmd.CombinedOutput()
		if healthErr == nil {
			s.t.Logf("Health check response: %s", string(healthOut))
			if strings.Contains(string(healthOut), `"healthy":true`) && strings.Contains(string(healthOut), `"block_height":1`) {
				s.t.Log("Node is healthy and has produced the first block")
				break
			}
		}
		if i == 29 {
			return errors.New("node failed to become healthy or produce the first block")
		}
		time.Sleep(time.Second)
	}

	// Create client
	s.t.Log("Creating private key...")
	pk, err := kwilcrypto.Secp256k1PrivateKeyFromHex(TestPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}
	s.t.Log("Successfully created private key")

	s.t.Log("Creating TN client...")
	s.t.Logf("Using provider: %s", TestKwilProvider)

	// Get the Ethereum address from the public key
	pubKeyBytes := pk.PubKey().Bytes()
	// Remove the first byte which is the compression flag
	pubKeyBytes = pubKeyBytes[1:]
	addr, err := util.NewEthereumAddressFromBytes(crypto.Keccak256(pubKeyBytes)[12:])
	if err != nil {
		return fmt.Errorf("failed to get address from public key: %w", err)
	}
	s.t.Logf("Using signer with address: %s", addr.Address())

	s.t.Log("Attempting to create client...")
	var lastErr error
	for i := 0; i < 60; i++ { // 60 seconds max wait
		s.t.Logf("Attempt %d/60: Creating client with provider URL %s", i+1, TestKwilProvider)

		// First check if the server is accepting connections
		cmd := exec.Command("curl", "-s", "-w", "\n%{http_code}", "http://localhost:8484/api/v1/health")
		out, err := cmd.CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("health check command failed: %w", err)
			s.t.Logf("Health check command failed: %v", err)
			time.Sleep(time.Second)
			continue
		}

		// Split output into response body and status code
		parts := strings.Split(string(out), "\n")
		if len(parts) != 2 {
			lastErr = fmt.Errorf("unexpected health check output format: %s", string(out))
			s.t.Logf("Health check output format error: %s", string(out))
			time.Sleep(time.Second)
			continue
		}

		statusCode := strings.TrimSpace(parts[1])
		s.t.Logf("Health check response - Status: %s", statusCode)
		if statusCode != "200" {
			lastErr = fmt.Errorf("health check returned non-200 status: %s", statusCode)
			s.t.Logf("Health check failed with status %s", statusCode)
			time.Sleep(time.Second)
			continue
		}

		s.t.Log("Health check passed, attempting to create client...")
		// Try to create the client now that we know the server is accepting connections
		s.client, err = tnclient.NewClient(
			s.ctx,
			TestKwilProvider,
			tnclient.WithSigner(&auth.EthPersonalSigner{Key: *pk}),
		)
		if err != nil {
			lastErr = fmt.Errorf("failed to create TN client: %w", err)
			s.t.Logf("Client creation failed: %v", err)
			time.Sleep(time.Second)
			continue
		}

		// Successfully created client
		s.t.Log("Client created successfully")
		break
	}

	if s.client == nil {
		return fmt.Errorf("failed to create client after 60 attempts. Last error: %w", lastErr)
	}

	return nil
}

// Teardown tears down the test environment
func (s *ServerFixture) Teardown() {
	// Stop containers in reverse order
	s.docker.stopContainer(s.containers.tsndb.name)
	s.docker.stopContainer(s.containers.postgres.name)

	// Tear down the network
	s.docker.teardownNetwork()

	// Clean up any other resources
	s.docker.cleanup()
}

// Client returns the client for interacting with the server
func (s *ServerFixture) Client() *tnclient.Client {
	return s.client
}

// newDocker creates a new docker helper
func newDocker(t *testing.T) *docker {
	return &docker{t: t}
}

// exec executes a command in a container
func (d *docker) exec(container string, args ...string) (string, error) {
	cmdArgs := append([]string{"exec", container}, args...)
	return d.run(cmdArgs...)
}

// run executes a docker command
func (d *docker) run(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// setupNetwork creates a docker network
func (d *docker) setupNetwork() error {
	d.run("network", "rm", networkName)
	_, err := d.run("network", "create", networkName)
	return err
}

// teardownNetwork removes the docker network
func (d *docker) teardownNetwork() error {
	_, err := d.run("network", "rm", networkName)
	return err
}

// startContainer starts a container with the given spec and waits for it to be healthy
func (d *docker) startContainer(spec containerSpec) error {
	args := []string{"run", "--rm", "--name", spec.name, "--network", networkName, "-d"}

	if spec.tmpfsPath != "" {
		args = append(args, "--tmpfs", spec.tmpfsPath)
	}

	for _, env := range spec.envVars {
		args = append(args, "-e", env)
	}

	if spec.name == "test-tsn-db" {
		args = append(args, "-p", "50051:50051", "-p", "50151:50151", "-p", "8080:8080",
			"-p", "8484:8484", "-p", "26656:26656", "-p", "26657:26657",
			"--entrypoint", "/app/kwild",
			spec.image,
			"--autogen",
			"--app.pg-db-host", "test-kwil-postgres",
			"--app.hostname", "test-tsn-db",
			"--chain.p2p.external-address", "http://test-tsn-db:26656",
		)
	} else {
		args = append(args, spec.image)
	}

	out, err := d.run(args...)
	if err != nil {
		return fmt.Errorf("failed to start container %s: %w\nOutput: %s", spec.name, err, out)
	}

	if spec.healthCheck != nil {
		err := pollUntilTrue(context.Background(), 10*time.Second, func() bool {
			return spec.healthCheck(d) == nil
		})
		if err != nil {
			if logs, logsErr := d.run("logs", spec.name); logsErr == nil {
				d.t.Logf("Container logs for %s:\n%s", spec.name, logs)
			}
			return fmt.Errorf("container %s failed to become healthy: %w", spec.name, err)
		}
	}

	if spec.name == "test-tsn-db" {
		err := pollUntilTrue(context.Background(), 30*time.Second, func() bool {
			out, err := exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "http://localhost:8484/api/v1/health").Output()
			if err != nil {
				return false
			}
			return strings.TrimSpace(string(out)) == "200"
		})
		if err != nil {
			if logs, logsErr := d.run("logs", spec.name); logsErr == nil {
				d.t.Logf("Container logs for %s:\n%s", spec.name, logs)
			}
			return fmt.Errorf("RPC server in container %s failed to become ready: %w", spec.name, err)
		}
	}

	return nil
}

// stopContainer stops a container
func (d *docker) stopContainer(name string) error {
	_, err := d.run("stop", name)
	if err != nil {
		return fmt.Errorf("failed to stop container %s: %w", name, err)
	}
	d.t.Logf("Stopped container %s", name)
	return nil
}

// cleanup removes all docker resources
func (d *docker) cleanup() {
	// Get all container IDs
	out, err := d.run("ps", "-aq")
	if err == nil && out != "" {
		containers := strings.Fields(out)
		if len(containers) > 0 {
			killArgs := append([]string{"kill"}, containers...)
			d.run(killArgs...)
			rmArgs := append([]string{"rm"}, containers...)
			d.run(rmArgs...)
		}
	}

	// Remove networks
	d.run("network", "prune", "-f")

	// Remove volume
	d.run("volume", "rm", "tsn-config")
}

// pollUntilTrue polls a condition until it returns true or a timeout is reached
func pollUntilTrue(ctx context.Context, timeout time.Duration, check func() bool) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return nil
		}
		time.Sleep(time.Second)
	}
	return errors.New("condition not met within timeout")
}
