package sftp

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Config represents SSH connection parameters.
type Config struct {
	Username     string
	Password     string
	PrivateKey   string
	Server       string
	KeyExchanges []string

	Timeout time.Duration
}

// Client provides basic functionality to interact with a SFTP server.
type Client struct {
	config     Config
	sshClient  *ssh.Client
	sftpClient *sftp.Client
}

// New initialises SSH and SFTP clients and returns Client type to use.
func New(config Config) (*Client, error) {
	c := &Client{
		config: config,
	}

	if err := c.connect(); err != nil {
		return nil, err
	}

	return c, nil
}

// Upload writes a file to a remote location.
func (c *Client) Upload(filePath string, fileContent []byte) error {
	if err := c.connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	file, err := c.sftpClient.Create(filePath)
	if err != nil {
		return fmt.Errorf("file create: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(fileContent); err != nil {
		return fmt.Errorf("file write: %w", err)
	}

	return nil
}

// Download returns a remote file.
func (c *Client) Download(filePath string) ([]byte, error) {
	if err := c.connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	file, err := c.sftpClient.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("file open: %w", err)
	}
	defer file.Close()

	return ioutil.ReadAll(file)
}

// Info gets the details of a file. If the file was not found, an error is returned.
func (c *Client) Info(filePath string) (os.FileInfo, error) {
	if err := c.connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	info, err := c.sftpClient.Lstat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file stats: %w", err)
	}

	return info, nil
}

// Close closes open connections.
func (c *Client) Close() {
	if c.sftpClient != nil {
		c.sftpClient.Close()
	}
	if c.sshClient != nil {
		c.sshClient.Close()
	}
}

// connect initialises a new SSH and SFTP client only if they were not
// initialised before at all and, they were initialised but the SSH
// connection was lost for any reason.
func (c *Client) connect() error {
	if c.sshClient != nil {
		_, _, err := c.sshClient.SendRequest("keepalive", false, nil)
		if err == nil {
			return nil
		}
	}

	auth := ssh.Password(c.config.Password)
	if c.config.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(c.config.PrivateKey))
		if err != nil {
			return fmt.Errorf("ssh parse private key: %w", err)
		}
		auth = ssh.PublicKeys(signer)
	}

	cfg := &ssh.ClientConfig{
		User: c.config.Username,
		Auth: []ssh.AuthMethod{
			auth,
		},
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error { return nil },
		Timeout:         c.config.Timeout,
		Config: ssh.Config{
			KeyExchanges: c.config.KeyExchanges,
		},
	}

	sshClient, err := ssh.Dial("tcp", c.config.Server, cfg)
	if err != nil {
		return fmt.Errorf("ssh dial: %w", err)
	}
	c.sshClient = sshClient

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("sftp new client: %w", err)
	}
	c.sftpClient = sftpClient

	return nil
}
