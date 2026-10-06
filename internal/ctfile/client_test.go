package ctfile

import "testing"

func TestParseLink(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantKind string
		wantKey  string
		wantPass string
		wantErr  bool
	}{
		{
			name:     "带提取码的文件链接",
			raw:      "https://url67.ctfile.com/f/65712267-17569899321195-e8ce7f?p=5577",
			wantKind: "f",
			wantKey:  "65712267-17569899321195-e8ce7f",
			wantPass: "5577",
		},
		{
			name:     "不带提取码的文件链接",
			raw:      "https://url67.ctfile.com/f/65712267-17569899321195-e8ce7f",
			wantKind: "f",
			wantKey:  "65712267-17569899321195-e8ce7f",
		},
		{
			name:     "其他域名 545c.com",
			raw:      "https://545c.com/f/12345-67890-abcdef",
			wantKind: "f",
			wantKey:  "12345-67890-abcdef",
		},
		{
			name:     "url.ctfile.com 分享链接",
			raw:      "https://url.ctfile.com/s/abc123?p=8899&fk=xx&d=1",
			wantKind: "s",
			wantKey:  "abc123",
			wantPass: "8899",
		},
		{
			name:     "目录链接",
			raw:      "https://url67.ctfile.com/d/65712267-17569899321195",
			wantKind: "d",
			wantKey:  "65712267-17569899321195",
		},
		{
			name:     "省略协议头",
			raw:      "url67.ctfile.com/f/65712267-17569899321195-e8ce7f?p=5577",
			wantKind: "f",
			wantKey:  "65712267-17569899321195-e8ce7f",
			wantPass: "5577",
		},
		{
			name:     "password 参数别名",
			raw:      "https://url67.ctfile.com/f/abc?password=geheim",
			wantKind: "f",
			wantKey:  "abc",
			wantPass: "geheim",
		},
		{
			name:    "App 分享码不支持",
			raw:     "ctfile://xturlVzAFblYwBzAFM1BDUHZQYFA2VWYFZ1A2",
			wantErr: true,
		},
		{
			name:    "非城通域名",
			raw:     "https://example.com/f/abc123",
			wantErr: true,
		},
		{
			name:    "空链接",
			raw:     "",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLink(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望报错，但解析成功: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if got.Kind != tc.wantKind {
				t.Errorf("Kind = %q, 期望 %q", got.Kind, tc.wantKind)
			}
			if got.FileKey != tc.wantKey {
				t.Errorf("FileKey = %q, 期望 %q", got.FileKey, tc.wantKey)
			}
			if got.Passcode != tc.wantPass {
				t.Errorf("Passcode = %q, 期望 %q", got.Passcode, tc.wantPass)
			}
		})
	}
}
