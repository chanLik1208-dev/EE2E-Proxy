// Package tunnelclient 是 client 端的核心邏輯，供 CLI 與 GUI 共用。
//
// 它主動連到 server、要求開放遠端端口，並把每個進來的訪客連線接到本機目標服務。
// 連線中斷會自動重連，直到 context 被取消。
package tunnelclient

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"reverse-tunnel/internal/protocol"
)

// ErrAuth 代表密鑰錯誤或遠端端口被占用等不該自動重試的致命錯誤。
var ErrAuth = errors.New("註冊失敗")

// Client 表示一個反向隧道客戶端。
type Client struct {
	Server     string                       // server 位址，例如 "example.com:7000"
	Token      string                       // 共享密鑰
	Local      string                       // 本機目標服務，例如 "127.0.0.1:25565"
	RemotePort int                          // 要在 server 公網開放的端口
	Logf       func(format string, a ...any) // 日誌輸出
}

func (c *Client) logf(format string, a ...any) {
	if c.Logf != nil {
		c.Logf(format, a...)
	}
}

// Run 持續維持隧道，斷線自動重連，直到 ctx 被取消。
// 若遇到 ErrAuth 這類致命錯誤則停止並回傳。
func (c *Client) Run(ctx context.Context) error {
	for {
		err := c.runOnce(ctx)
		if ctx.Err() != nil {
			c.logf("已停止")
			return nil
		}
		if errors.Is(err, ErrAuth) {
			return err
		}
		if err != nil {
			c.logf("連線中斷: %v，5 秒後重試", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *Client) runOnce(ctx context.Context) error {
	var d net.Dialer
	ctrl, err := d.DialContext(ctx, "tcp", c.Server)
	if err != nil {
		return err
	}
	defer ctrl.Close()

	// ctx 取消時關閉控制連線，讓底下的 ReadMsg 解除阻塞。
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			ctrl.Close()
		case <-stop:
		}
	}()

	if err := protocol.WriteMsg(ctrl, &protocol.Msg{
		Type:       protocol.TypeRegister,
		Token:      c.Token,
		RemotePort: c.RemotePort,
	}); err != nil {
		return err
	}

	resp, err := protocol.ReadMsg(ctrl)
	if err != nil {
		return err
	}
	if resp.Type != protocol.TypeOK {
		c.logf("註冊失敗: %s", resp.Error)
		return ErrAuth
	}
	c.logf("已連上 server，公網端口 %d -> 本機 %s", c.RemotePort, c.Local)

	for {
		msg, err := protocol.ReadMsg(ctrl)
		if err != nil {
			return err
		}
		if msg.Type == protocol.TypeNewConn {
			go c.handleNewConn(ctx, msg.ConnID)
		}
	}
}

func (c *Client) handleNewConn(ctx context.Context, id uint64) {
	var d net.Dialer
	dataConn, err := d.DialContext(ctx, "tcp", c.Server)
	if err != nil {
		c.logf("資料連線回連失敗: %v", err)
		return
	}
	if err := protocol.WriteMsg(dataConn, &protocol.Msg{
		Type:   protocol.TypeData,
		Token:  c.Token,
		ConnID: id,
	}); err != nil {
		dataConn.Close()
		return
	}

	localConn, err := d.DialContext(ctx, "tcp", c.Local)
	if err != nil {
		c.logf("無法連到本機服務 %s: %v", c.Local, err)
		dataConn.Close()
		return
	}
	pipe(dataConn, localConn)
}

func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); io.Copy(a, b); a.Close(); b.Close() }()
	go func() { defer wg.Done(); io.Copy(b, a); a.Close(); b.Close() }()
	wg.Wait()
}
