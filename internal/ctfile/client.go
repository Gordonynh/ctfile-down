package ctfile

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	baseAPI   = "https://webapi.ctfile.com"
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
)

// Link is a parsed ctfile share link.
type Link struct {
	FileKey  string
	Passcode string
	PageURL  string
}

// FileInfo holds metadata about a resolved ctfile file.
type FileInfo struct {
	FileID      int64  `json:"file_id"`
	FileName    string `json:"file_name"`
	SizeDisplay string `json:"size_display"`
	FileChk     string `json:"file_chk"`
	UserID      int64  `json:"user_id"`
	StartTime   int64  `json:"start_time"`
	WaitSeconds int64  `json:"wait_seconds"`
	VerifyCode  string `json:"verify_code"`
}

// Ticket is a one-shot direct download link.
type Ticket struct {
	URL      string
	FileName string
	FileSize int64
}

// Client talks to the ctfile web API.
type Client struct {
	http *http.Client
	link Link
}

// New creates a Client for the given link.
func New(link Link) *Client {
	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   8,
	}
	return &Client{
		http: &http.Client{Transport: transport},
		link: link,
	}
}

// ParseLink parses a ctfile share URL such as
// https://url67.ctfile.com/f/65712267-17569899321195-e8ce7f?p=5577
func ParseLink(raw string) (Link, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return Link{}, fmt.Errorf("无效链接: %w", err)
	}
	if !strings.Contains(strings.ToLower(u.Host), "ctfile.com") {
		return Link{}, fmt.Errorf("不是 ctfile.com 链接: %s", u.Host)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || !strings.HasPrefix(parts[0], "f") {
		return Link{}, fmt.Errorf("无法识别的 ctfile 链接格式（应为 /f/<id>）")
	}
	return Link{
		FileKey:  parts[1],
		Passcode: u.Query().Get("p"),
		PageURL:  u.String(),
	}, nil
}

func (c *Client) getJSON(u string, out any) error {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	if c.link.PageURL != "" {
		req.Header.Set("Referer", c.link.PageURL)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}
	return json.Unmarshal(body, out)
}

// Resolve fetches file metadata for the link.
func (c *Client) Resolve() (*FileInfo, error) {
	u := fmt.Sprintf("%s/getfile.php?path=f&f=%s&passcode=%s&r=%d&ref=&url=%s",
		baseAPI,
		url.QueryEscape(c.link.FileKey),
		url.QueryEscape(c.link.Passcode),
		time.Now().UnixNano(),
		url.QueryEscape(c.link.PageURL),
	)

	var raw struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		File    struct {
			FileID      int64  `json:"file_id"`
			FileName    string `json:"file_name"`
			FileSize    string `json:"file_size"`
			FileChk     string `json:"file_chk"`
			UserID      int64  `json:"userid"`
			StartTime   int64  `json:"start_time"`
			WaitSeconds int64  `json:"wait_seconds"`
			VerifyCode  string `json:"verifycode"`
		} `json:"file"`
	}
	if err := c.getJSON(u, &raw); err != nil {
		return nil, fmt.Errorf("解析文件失败: %w", err)
	}
	if raw.Code != 200 || raw.File.FileID == 0 {
		if raw.Message != "" {
			return nil, fmt.Errorf("解析失败: %s", raw.Message)
		}
		return nil, fmt.Errorf("解析失败 (code %d)，请检查链接与提取码", raw.Code)
	}
	return &FileInfo{
		FileID:      raw.File.FileID,
		FileName:    raw.File.FileName,
		SizeDisplay: raw.File.FileSize,
		FileChk:     raw.File.FileChk,
		UserID:      raw.File.UserID,
		StartTime:   raw.File.StartTime,
		WaitSeconds: raw.File.WaitSeconds,
		VerifyCode:  raw.File.VerifyCode,
	}, nil
}

// GetDownloadURL obtains a one-shot direct download link.
func (c *Client) GetDownloadURL(info *FileInfo) (*Ticket, error) {
	u := fmt.Sprintf("%s/get_down_url.php?uid=%d&fid=%d&file_chk=%s&start_time=%d&wait_seconds=%d&rd=%d",
		baseAPI,
		info.UserID,
		info.FileID,
		url.QueryEscape(info.FileChk),
		info.StartTime,
		info.WaitSeconds,
		time.Now().UnixNano(),
	)

	var raw struct {
		Code     int    `json:"code"`
		DownURL  string `json:"downurl"`
		FileSize int64  `json:"file_size"`
		FileName string `json:"file_name"`
		Message  string `json:"message"`
	}
	if err := c.getJSON(u, &raw); err != nil {
		return nil, fmt.Errorf("获取下载地址失败: %w", err)
	}
	if raw.Code != 200 || raw.DownURL == "" {
		return nil, fmt.Errorf("获取下载地址失败 (code %d): %s", raw.Code, raw.Message)
	}
	return &Ticket{URL: raw.DownURL, FileName: raw.FileName, FileSize: raw.FileSize}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
