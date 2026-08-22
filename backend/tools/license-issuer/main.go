// Command license-issuer 是厂商侧的离线授权签发工具。
package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Fanc1er/Kate/backend/internal/master/license"
)

func main() {
	var (
		keyPath    = flag.String("key", "", "厂商 RSA 私钥路径 (PEM)")
		machine    = flag.String("machine", "", "机器码 (hex)")
		notBefore  = flag.String("not-before", "", "延迟激活时间 (YYYY-MM-DD)")
		notAfter   = flag.String("not-after", "", "授权截止时间 (YYYY-MM-DD)")
		maxAssets  = flag.Int("max-assets", 0, "资产数量上限 (0=不限)")
		maxWorkers = flag.Int("max-workers", 0, "Worker 节点数上限 (0=不限)")
		customer   = flag.String("customer", "", "客户标识")
		out        = flag.String("out", "license.lic", "输出文件路径")
	)
	flag.Parse()

	if *keyPath == "" || *machine == "" || *notAfter == "" {
		fmt.Fprintln(os.Stderr, "usage: license-issuer -key private.pem -machine <hex> -not-after YYYY-MM-DD [-not-before YYYY-MM-DD] [-max-assets N] [-max-workers N] [-customer NAME] [-out license.lic]")
		os.Exit(2)
	}

	priv, err := loadPrivateKey(*keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load key: %v\n", err)
		os.Exit(1)
	}

	nb := time.Time{}
	if *notBefore != "" {
		if nb, err = time.Parse("2006-01-02", *notBefore); err != nil {
			fmt.Fprintf(os.Stderr, "parse not-before: %v\n", err)
			os.Exit(1)
		}
	}
	na, err := time.Parse("2006-01-02", *notAfter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse not-after: %v\n", err)
		os.Exit(1)
	}

	data, err := license.Issue(*machine, license.IssueOptions{
		NotBefore: nb,
		NotAfter:  na,
		MaxAssets: *maxAssets,
		MaxWorkers: *maxWorkers,
		Customer:  *customer,
	}, priv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "issue: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("license written to %s\n", *out)
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM")
	}
	if priv, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return priv, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA private key")
	}
	return priv, nil
}
