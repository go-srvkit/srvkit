// Copyright © 2025 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package qnet

import (
	"net"
	"net/http"
	"strings"
)

// JSONResponse HTTP响应
type JSONResponse struct {
	Code    int    `json:"code"`
	Message string `json:"msg,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// GetHTTPRequestIP 获取http请求的来源IP
func GetHTTPRequestIP(req *http.Request) string {
	ip := req.Header.Get("X-Forwarded-For")
	ip = strings.TrimSpace(strings.Split(ip, ",")[0])
	if ip == "" {
		realIP := req.Header.Get("X-Real-IP")
		ip = strings.TrimSpace(realIP)
	}
	if ip != "" {
		return ip
	}
	if ip, _, err := net.SplitHostPort(strings.TrimSpace(req.RemoteAddr)); err == nil {
		return ip
	}
	return ""
}
