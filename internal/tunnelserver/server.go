// Package tunnelserver 是 server 端的核心邏輯，供 CLI 與 GUI 共用。
//
// 它在 Bind 指定的端口上同時接收控制連線與資料連線，並可由 context 取消來停止。
package tunnelserver

import (
	"context"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"reverse-tunnel/internal/protocol"
)

// Server 表示一個反向隧道伺服器。建立後設定欄位再呼叫 ListenAndServe。
type Server struct {
	Bind  string                       // 控制/資料連線監聽位址，例如 ":7000"
	Token string                       // 共享密鑰
	Logf  func(format string, a ...any) // 日誌輸出（CLI 用 log，GUI 導到視窗）

	mu      sync.Mutex
	pending map[uint64]chan net.Conn
	nextID  uint64
}

func (s *Server) logf(format string, a ...any) {
	if s.Logf != nil {
		s.Logf(format, a...)
	}
}

// ListenAndServe 啟動伺服器並阻塞，直到 ctx 被取消或發生致命錯誤。
func (s *Server) ListenAndServe(ctx context.Context) error {
	s.pending = make(map[uint64]chan net.Conn)

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", s.Bind)
	if err != nil {
		return err
	}
	s.logf("server 已啟動，監聽控制端口 %s", s.Bind)

	// ctx 取消時關閉主監聽，讓 Accept 解除阻塞。
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				s.logf("server 已停止")
				return nil
			}
			s.logf("accept 失敗: %v", err)
			continue
		}
		go s.handle(ctx, conn)
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	msg, err := protocol.ReadMsg(conn)
	if err != nil {
		conn.Close()
		return
	}
	if msg.Token != s.Token {
		_ = protocol.WriteMsg(conn, &protocol.Msg{Type: protocol.TypeError, Error: "認證失敗"})
		conn.Close()
		return
	}
	switch msg.Type {
	case protocol.TypeRegister:
		s.handleControl(ctx, conn, msg)
	case protocol.TypeData:
		s.handleData(conn, msg)
	default:
		conn.Close()
	}
}

func (s *Server) handleControl(ctx context.Context, ctrl net.Conn, msg *protocol.Msg) {
	defer ctrl.Close()

	pub, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(msg.RemotePort)))
	if err != nil {
		_ = protocol.WriteMsg(ctrl, &protocol.Msg{Type: protocol.TypeError, Error: "無法開放端口: " + err.Error()})
		return
	}
	defer pub.Close()

	if err := protocol.WriteMsg(ctrl, &protocol.Msg{Type: protocol.TypeOK}); err != nil {
		return
	}
	s.logf("已開放公網端口 %d", msg.RemotePort)

	// 控制連線斷開、或伺服器整體停止時，關閉這個公網監聽。
	done := make(chan struct{})
	go func() {
		io.Copy(io.Discard, ctrl) // client 不會在控制連線上再送資料，讀到 EOF 即代表離線
		close(done)
		pub.Close()
	}()
	go func() {
		select {
		case <-ctx.Done():
			pub.Close()
		case <-done:
		}
	}()

	for {
		visitor, err := pub.Accept()
		if err != nil {
			select {
			case <-done:
				s.logf("client 離線，關閉公網端口 %d", msg.RemotePort)
			default:
				if ctx.Err() == nil {
					s.logf("公網端口 %d accept 失敗: %v", msg.RemotePort, err)
				}
			}
			return
		}

		id := atomic.AddUint64(&s.nextID, 1)
		ch := make(chan net.Conn, 1)
		s.mu.Lock()
		s.pending[id] = ch
		s.mu.Unlock()

		if err := protocol.WriteMsg(ctrl, &protocol.Msg{Type: protocol.TypeNewConn, ConnID: id}); err != nil {
			s.cleanup(id)
			visitor.Close()
			return
		}
		go s.pairVisitor(id, visitor, ch)
	}
}

func (s *Server) pairVisitor(id uint64, visitor net.Conn, ch chan net.Conn) {
	defer visitor.Close()
	select {
	case dataConn := <-ch:
		pipe(visitor, dataConn)
	case <-time.After(10 * time.Second):
		s.logf("訪客連線 %d 等不到 client 回連，放棄", id)
		s.cleanup(id)
	}
}

func (s *Server) handleData(conn net.Conn, msg *protocol.Msg) {
	s.mu.Lock()
	ch, ok := s.pending[msg.ConnID]
	if ok {
		delete(s.pending, msg.ConnID)
	}
	s.mu.Unlock()
	if !ok {
		conn.Close()
		return
	}
	ch <- conn
}

func (s *Server) cleanup(id uint64) {
	s.mu.Lock()
	delete(s.pending, id)
	s.mu.Unlock()
}

func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); io.Copy(a, b); a.Close(); b.Close() }()
	go func() { defer wg.Done(); io.Copy(b, a); a.Close(); b.Close() }()
	wg.Wait()
}
