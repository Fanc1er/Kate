package utils

import "testing"

func TestSHA256Hex(t *testing.T) {
	if got := SHA256Hex([]byte("abc")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("unexpected sha256: %s", got)
	}
}

func TestMD5Hex(t *testing.T) {
	if got := MD5Hex("abc"); got != "900150983cd24fb0d6963f7d28e17f72" {
		t.Fatalf("unexpected md5: %s", got)
	}
}

func TestHashAndCheckPassword(t *testing.T) {
	pwd := "Test@123456Strong"
	hash, err := HashPassword(pwd)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !CheckPassword(hash, pwd) {
		t.Fatal("CheckPassword 应通过")
	}
	if CheckPassword(hash, "WrongPassword1!") {
		t.Fatal("CheckPassword 不应通过错误密码")
	}
	// 相同密码两次哈希不同（随机盐）。
	hash2, _ := HashPassword(pwd)
	if hash == hash2 {
		t.Fatal("相同密码不应产生相同 bcrypt 哈希")
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Example.COM", "http://example.com/"},
		{"https://example.com", "https://example.com/"},
		{"http://example.com:80/foo", "http://example.com/foo"},
		{"https://example.com:443/foo", "https://example.com/foo"},
		{"http://example.com/path?q=1#frag", "http://example.com/path"},
		{"http://example.com/path/", "http://example.com/path"},
	}
	for _, c := range cases {
		got, err := NormalizeURL(c.in)
		if err != nil {
			t.Fatalf("NormalizeURL(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("NormalizeURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeURLReject(t *testing.T) {
	for _, in := range []string{"", "ftp://example.com", "javascript:alert(1)"} {
		if _, err := NormalizeURL(in); err == nil {
			t.Fatalf("NormalizeURL(%q) 应报错", in)
		}
	}
}

func TestURLKeyStable(t *testing.T) {
	a, err := NormalizeURL("http://example.com/")
	if err != nil {
		t.Fatalf("NormalizeURL: %v", err)
	}
	b, err := NormalizeURL("HTTP://EXAMPLE.COM/")
	if err != nil {
		t.Fatalf("NormalizeURL: %v", err)
	}
	if a != b {
		t.Fatalf("归一化后应一致: %q vs %q", a, b)
	}
	if URLKey(a) != URLKey(b) {
		t.Fatal("URL 去重键应不区分大小写")
	}
}
