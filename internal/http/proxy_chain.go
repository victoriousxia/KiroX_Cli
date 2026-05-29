package http

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// ProxyChain starts a local SOCKS5 server that chains two proxies.
// Traffic flow: client → local SOCKS5 → primaryProxy → upstreamProxy → target
// Returns the local address (e.g. "127.0.0.1:xxxxx") and a stop function.
func ProxyChain(primaryProxy, upstreamProxy string) (localAddr string, stop func(), err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("listen failed: %w", err)
	}

	primaryDialer, err := socksDialerFromURL(primaryProxy)
	if err != nil {
		listener.Close()
		return "", nil, fmt.Errorf("primary proxy invalid: %w", err)
	}

	upstreamURL, err := url.Parse(upstreamProxy)
	if err != nil {
		listener.Close()
		return "", nil, fmt.Errorf("upstream proxy URL invalid: %w", err)
	}
	upstreamHost := upstreamURL.Host

	var wg sync.WaitGroup
	done := make(chan struct{})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					continue
				}
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				handleSocks5(conn, primaryDialer, upstreamHost, upstreamURL.User)
			}()
		}
	}()

	addr := listener.Addr().String()
	stopFn := func() {
		close(done)
		listener.Close()
		wg.Wait()
	}

	log.Printf("[代理链] 本地转发 %s → %s → %s", addr, primaryProxy, upstreamProxy)
	return addr, stopFn, nil
}

func socksDialerFromURL(proxyURL string) (proxy.Dialer, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	var auth *proxy.Auth
	if u.User != nil {
		auth = &proxy.Auth{User: u.User.Username()}
		if p, ok := u.User.Password(); ok {
			auth.Password = p
		}
	}

	return proxy.SOCKS5("tcp", u.Host, auth, proxy.Direct)
}

// handleSocks5 implements a minimal SOCKS5 server that forwards connections
// through the primary proxy to the upstream SOCKS5 proxy, then to the target.
func handleSocks5(conn net.Conn, primaryDialer proxy.Dialer, upstreamHost string, upstreamUser *url.Userinfo) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(60 * time.Second))

	// SOCKS5 handshake: read client greeting
	buf := make([]byte, 258)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	if buf[0] != 0x05 {
		return
	}
	nMethods := int(buf[1])
	if _, err := io.ReadFull(conn, buf[:nMethods]); err != nil {
		return
	}
	// Reply: no auth required
	conn.Write([]byte{0x05, 0x00})

	// Read CONNECT request
	if _, err := io.ReadFull(conn, buf[:4]); err != nil {
		return
	}
	if buf[0] != 0x05 || buf[1] != 0x01 {
		conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	var targetAddr string
	switch buf[3] {
	case 0x01: // IPv4
		if _, err := io.ReadFull(conn, buf[:4+2]); err != nil {
			return
		}
		targetAddr = fmt.Sprintf("%d.%d.%d.%d:%d", buf[0], buf[1], buf[2], buf[3], binary.BigEndian.Uint16(buf[4:6]))
	case 0x03: // Domain
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return
		}
		domainLen := int(buf[0])
		if _, err := io.ReadFull(conn, buf[:domainLen+2]); err != nil {
			return
		}
		targetAddr = fmt.Sprintf("%s:%d", string(buf[:domainLen]), binary.BigEndian.Uint16(buf[domainLen:domainLen+2]))
	case 0x04: // IPv6
		if _, err := io.ReadFull(conn, buf[:16+2]); err != nil {
			return
		}
		ip := net.IP(buf[:16])
		port := binary.BigEndian.Uint16(buf[16:18])
		targetAddr = fmt.Sprintf("[%s]:%d", ip.String(), port)
	default:
		conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	// Connect to upstream SOCKS5 proxy through primary proxy
	upstreamConn, err := primaryDialer.Dial("tcp", upstreamHost)
	if err != nil {
		conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer upstreamConn.Close()

	// Perform SOCKS5 handshake with upstream proxy
	if err := upstreamSocks5Connect(upstreamConn, targetAddr, upstreamUser); err != nil {
		conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	// Success reply to client
	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	conn.SetDeadline(time.Time{})

	// Relay data
	relay(conn, upstreamConn)
}

// upstreamSocks5Connect performs SOCKS5 handshake with the upstream proxy
func upstreamSocks5Connect(conn net.Conn, target string, user *url.Userinfo) error {
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	var authMethod byte = 0x00
	if user != nil {
		authMethod = 0x02
	}

	// Greeting
	conn.Write([]byte{0x05, 0x01, authMethod})

	buf := make([]byte, 258)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return fmt.Errorf("upstream greeting failed: %w", err)
	}
	if buf[0] != 0x05 {
		return fmt.Errorf("upstream not SOCKS5")
	}

	// Username/password auth if needed
	if buf[1] == 0x02 && user != nil {
		username := user.Username()
		password, _ := user.Password()
		authReq := []byte{0x01, byte(len(username))}
		authReq = append(authReq, []byte(username)...)
		authReq = append(authReq, byte(len(password)))
		authReq = append(authReq, []byte(password)...)
		conn.Write(authReq)

		if _, err := io.ReadFull(conn, buf[:2]); err != nil {
			return fmt.Errorf("upstream auth failed: %w", err)
		}
		if buf[1] != 0x00 {
			return fmt.Errorf("upstream auth rejected")
		}
	} else if buf[1] == 0xFF {
		return fmt.Errorf("upstream no acceptable auth")
	}

	// CONNECT request
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return err
	}
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	req = append(req, []byte(host)...)
	req = append(req, byte(port>>8), byte(port&0xff))
	conn.Write(req)

	if _, err := io.ReadFull(conn, buf[:4]); err != nil {
		return fmt.Errorf("upstream connect failed: %w", err)
	}
	if buf[1] != 0x00 {
		return fmt.Errorf("upstream connect rejected: %d", buf[1])
	}

	// Skip bind address
	switch buf[3] {
	case 0x01:
		io.ReadFull(conn, buf[:4+2])
	case 0x03:
		io.ReadFull(conn, buf[:1])
		io.ReadFull(conn, buf[:int(buf[0])+2])
	case 0x04:
		io.ReadFull(conn, buf[:16+2])
	}

	conn.SetDeadline(time.Time{})
	return nil
}

func relay(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(b, a)
		b.(*net.TCPConn).CloseWrite()
	}()
	go func() {
		defer wg.Done()
		io.Copy(a, b)
		a.(*net.TCPConn).CloseWrite()
	}()
	wg.Wait()
}
