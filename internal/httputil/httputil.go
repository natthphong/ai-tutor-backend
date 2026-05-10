package httputil

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"time"
)

func InitHttpClient(timeout time.Duration, maxIdleConn, maxIdleConnPerHost, maxConnPerHost int) *http.Client {
	certPool := x509.NewCertPool()
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            certPool,
				InsecureSkipVerify: true,
			},
			MaxIdleConns:        maxIdleConn,
			MaxIdleConnsPerHost: maxIdleConnPerHost,
			MaxConnsPerHost:     maxConnPerHost,
		},
	}
	return client
}

type HTTPPostRequestFunc func(reqBody interface{}, url string) ([]byte, error)

func NewHttpPostCall(client *http.Client) HTTPPostRequestFunc {
	return func(reqBody interface{}, url string) ([]byte, error) {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create POST request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("POST request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, errors.New("POST request returned non-2xx status")
		}

		return body, nil
	}
}

type HTTPGetRequestFunc func(url string) ([]byte, error)

func NewHttpGetCall(client *http.Client) HTTPGetRequestFunc {
	return func(url string) ([]byte, error) {

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create GET request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("GET request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, errors.New("GET request returned non-2xx status")
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		return body, nil
	}
}
