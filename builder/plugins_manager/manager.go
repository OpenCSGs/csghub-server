package pluginmanager

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

// Definition describes a reusable host-side plugin connection.
type Definition struct {
	Name            string
	Command         []string
	Env             []string
	HandshakeConfig plugin.HandshakeConfig
	Plugin          plugin.Plugin
}

// Client owns a started plugin process and the API dispensed from it.
type Client struct {
	client *plugin.Client
	api    any
}

// API returns the client-side API produced by the plugin implementation.
func (c *Client) API() any {
	return c.api
}

// Close terminates the plugin process.
func (c *Client) Close() error {
	c.client.Kill()
	return nil
}

// Manager starts and caches plugin clients for the lifetime of a process.
type Manager struct {
	mu      sync.Mutex
	clients map[string]*Client
}

// NewManager creates an empty plugin client manager.
func NewManager() *Manager {
	return &Manager{clients: make(map[string]*Client)}
}

var defaultManager = NewManager()

// Default returns the process-wide manager used by long-running services.
func Default() *Manager {
	return defaultManager
}

// Get starts the plugin if needed and returns its dispensed API.
func (m *Manager) Get(def Definition) (any, error) {
	if def.Name == "" {
		return nil, fmt.Errorf("plugin name is required")
	}
	if def.Plugin == nil {
		return nil, fmt.Errorf("plugin implementation is required for %s", def.Name)
	}
	if len(def.Command) == 0 || def.Command[0] == "" {
		return nil, fmt.Errorf("plugin command is required for %s", def.Name)
	}

	key, err := definitionKey(def)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if client, ok := m.clients[key]; ok {
		if client.client.Exited() {
			client.Close()
			delete(m.clients, key)
		} else {
			return client.API(), nil
		}
	}

	client, err := startClient(def)
	if err != nil {
		return nil, err
	}
	m.clients[key] = client
	return client.API(), nil
}

// Close terminates all plugin processes owned by this manager.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, client := range m.clients {
		client.Close()
	}
	m.clients = make(map[string]*Client)
	return nil
}

func startClient(def Definition) (*Client, error) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   fmt.Sprintf("plugin_%s", def.Name),
		Output: os.Stdout,
		Level:  hclog.Debug,
	})

	command := exec.Command(def.Command[0], def.Command[1:]...)
	if len(def.Env) > 0 {
		command.Env = def.Env
	}
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  def.HandshakeConfig,
		Plugins:          map[string]plugin.Plugin{def.Name: def.Plugin},
		Cmd:              command,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		AutoMTLS:         true,
		SkipHostEnv:      true,
		Logger:           logger,
	})

	protocolClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("start plugin %s: %w", def.Name, err)
	}

	api, err := protocolClient.Dispense(def.Name)
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("dispense plugin %s: %w", def.Name, err)
	}

	return &Client{client: client, api: api}, nil
}

func definitionKey(def Definition) (string, error) {
	for _, arg := range def.Command {
		if arg == "" {
			return "", fmt.Errorf("plugin command for %s contains an empty argument", def.Name)
		}
	}

	env := make([]string, len(def.Env))
	copy(env, def.Env)
	sort.Strings(env)
	command := make([]string, len(def.Command))
	copy(command, def.Command)
	return strings.Join([]string{
		def.Name,
		def.HandshakeConfig.MagicCookieKey,
		def.HandshakeConfig.MagicCookieValue,
	}, "\x1f"), nil
}
