package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lucaswoodzy/pvetop/internal/models"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	ticket     string
	csrfToken  string
	token      string 
}

func NewClient(host, port string) *Client {
	return &Client{
		baseURL: fmt.Sprintf("https://%s:%s/api2/json", host, port),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func NewClientWithToken(host, port, token string) *Client {
	return &Client{
		baseURL: fmt.Sprintf("https://%s:%s/api2/json", host, port),
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (c *Client) Login(username, password string) error {
	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)

	req, err := http.NewRequest("POST", c.baseURL+"/access/ticket", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Ticket              string `json:"ticket"`
			CSRFPreventionToken string `json:"CSRFPreventionToken"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	c.ticket = result.Data.Ticket
	c.csrfToken = result.Data.CSRFPreventionToken

	return nil
}

func (c *Client) CreateAPIToken(username, tokenID string) (string, error) {
	if c.ticket == "" {
		return "", fmt.Errorf("not authenticated - call Login first")
	}

	data := url.Values{}
	data.Set("tokenid", tokenID)
	data.Set("privsep", "0") 

	path := fmt.Sprintf("/access/users/%s/token/%s", url.PathEscape(username), url.PathEscape(tokenID))
	resp, err := c.doRequest("POST", path, data)
	if err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to create token: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Value string `json:"value"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	fullToken := fmt.Sprintf("%s!%s=%s", username, tokenID, result.Data.Value)
	return fullToken, nil
}

func (c *Client) DeleteAPIToken(username, tokenID string) error {
	if c.ticket == "" && c.token == "" {
		return fmt.Errorf("not authenticated")
	}

	path := fmt.Sprintf("/access/users/%s/token/%s", url.PathEscape(username), url.PathEscape(tokenID))
	resp, err := c.doRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to delete token: HTTP %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) doRequest(method, path string, data url.Values) (*http.Response, error) {
	var req *http.Request
	var err error

	if data != nil && method != "GET" {
		req, err = http.NewRequest(method, c.baseURL+path, strings.NewReader(data.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, err = http.NewRequest(method, c.baseURL+path, nil)
		if err != nil {
			return nil, err
		}
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.token))
	} else {
		req.Header.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", c.ticket))
		req.Header.Set("CSRFPreventionToken", c.csrfToken)
	}

	return c.httpClient.Do(req)
}

func (c *Client) GetNodes() ([]models.Node, error) {
	resp, err := c.doRequest("GET", "/nodes", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []models.Node `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func (c *Client) GetVMs(node string) ([]models.Guest, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/nodes/%s/qemu", node), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []models.Guest `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	for i := range result.Data {
		result.Data[i].Type = "qemu"
		result.Data[i].Node = node
		if result.Data[i].Status == "running" {
			if status, err := c.GetVMStatus(node, result.Data[i].VMID); err == nil {
				result.Data[i].DiskRead = status.DiskRead
				result.Data[i].DiskWrite = status.DiskWrite
				result.Data[i].NetIn = status.NetIn
				result.Data[i].NetOut = status.NetOut
			}
		}
	}

	return result.Data, nil
}

func (c *Client) GetContainers(node string) ([]models.Guest, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/nodes/%s/lxc", node), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []models.Guest `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	for i := range result.Data {
		result.Data[i].Type = "lxc"
		result.Data[i].Node = node
		if result.Data[i].Status == "running" {
			if status, err := c.GetContainerStatus(node, result.Data[i].VMID); err == nil {
				result.Data[i].DiskRead = status.DiskRead
				result.Data[i].DiskWrite = status.DiskWrite
				result.Data[i].NetIn = status.NetIn
				result.Data[i].NetOut = status.NetOut
			}
		}
	}

	return result.Data, nil
}

func (c *Client) GetVMStatus(node string, vmid int) (*models.GuestStatus, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/nodes/%s/qemu/%d/status/current", node, vmid), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data models.GuestStatus `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func (c *Client) GetContainerStatus(node string, vmid int) (*models.GuestStatus, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/nodes/%s/lxc/%d/status/current", node, vmid), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data models.GuestStatus `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func (c *Client) GetAllGuests() ([]models.Guest, error) {
	nodes, err := c.GetNodes()
	if err != nil {
		return nil, err
	}

	var allGuests []models.Guest

	for _, node := range nodes {
		vms, err := c.GetVMs(node.Node)
		if err != nil {
			continue
		}
		allGuests = append(allGuests, vms...)

		containers, err := c.GetContainers(node.Node)
		if err != nil {
			continue
		}
		allGuests = append(allGuests, containers...)
	}

	return allGuests, nil
}
