package ctfile

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// maxFastBytes is the per-ticket fast-download allowance observed on the
// official direct-link CDN (~15.9 MB) before it throttles down to ~100 KB/s.
// Keeping each segment below this threshold keeps every segment in the fast
// zone. See the download engine notes in README for details.
const maxFastBytes int64 = 15_000_000

const maxRetries = 3

// ProgressFunc reports cumulative downloaded bytes.
type ProgressFunc func(downloaded, total int64)

// Download fetches the file to dstPath using segmented range requests. A fresh
// direct link is obtained for every segment, which resets the CDN throttle
// allowance; the official direct-link CDN otherwise limits a single link's
// sustained speed once the fast allowance is exhausted.
func (c *Client) Download(ctx context.Context, info *FileInfo, dstPath string, progress ProgressFunc) error {
	t, err := c.GetDownloadURL(info)
	if err != nil {
		return fmt.Errorf("获取下载地址失败: %w", err)
	}
	total := t.FileSize
	if total <= 0 {
		return fmt.Errorf("无法确定文件大小")
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	type seg struct{ start, end int64 }
	var segs []seg
	for start := int64(0); start < total; start += maxFastBytes {
		end := start + maxFastBytes - 1
		if end >= total {
			end = total - 1
		}
		segs = append(segs, seg{start, end})
	}

	tmpDir := dstPath + ".parts"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	var done int64
	for i, s := range segs {
		p := filepath.Join(tmpDir, fmt.Sprintf("seg_%04d", i))
		if err := c.downloadSegment(ctx, info, s.start, s.end, p); err != nil {
			return fmt.Errorf("分段 %d/%d 下载失败: %w", i+1, len(segs), err)
		}
		done += s.end - s.start + 1
		if progress != nil {
			progress(done, total)
		}
	}

	// Merge segments in order.
	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()
	for i := range segs {
		f, err := os.Open(filepath.Join(tmpDir, fmt.Sprintf("seg_%04d", i)))
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, f); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	return nil
}

func (c *Client) downloadSegment(ctx context.Context, info *FileInfo, start, end int64, path string) error {
	expect := end - start + 1
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		t, err := c.GetDownloadURL(info)
		if err != nil {
			lastErr = err
			continue
		}
		if err := c.fetchRange(ctx, t.URL, start, end, path); err != nil {
			lastErr = err
			os.Remove(path)
			continue
		}
		if st, err := os.Stat(path); err == nil && st.Size() == expect {
			return nil
		}
		lastErr = fmt.Errorf("分段大小不符: 期望 %d 字节", expect)
		os.Remove(path)
	}
	return lastErr
}

func (c *Client) fetchRange(ctx context.Context, urlStr string, start, end int64, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	if c.link.PageURL != "" {
		req.Header.Set("Referer", c.link.PageURL)
	}
	// The data CDN only serves the file when a Range header is present;
	// without it, it returns a 503 anti-leech page.
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	req.Header.Set("Accept", "*/*")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
