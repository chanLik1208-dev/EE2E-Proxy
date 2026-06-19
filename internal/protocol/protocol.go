// Package protocol 定義 server 與 client 之間溝通用的訊息格式。
//
// 每一條 TCP 連線在建立後，第一件事就是送出一條 JSON 控制訊息，
// 用 4 bytes 大端序的長度前綴包起來。server 靠第一條訊息的 Type
// 判斷這條連線是「控制連線(register)」還是「資料連線(data)」。
package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// 訊息類型
const (
	TypeRegister = "register" // client -> server：要求開放一個遠端端口
	TypeData     = "data"     // client -> server：這是一條資料連線，配對 ConnID
	TypeOK       = "ok"       // server -> client：註冊成功
	TypeError    = "error"    // server -> client：註冊失敗
	TypeNewConn  = "new_conn" // server -> client：有訪客連進來了，請建立資料連線
)

// Msg 是控制通道上傳遞的單一訊息。
type Msg struct {
	Type       string `json:"type"`
	Token      string `json:"token,omitempty"`       // 認證用的共享密鑰
	RemotePort int    `json:"remote_port,omitempty"` // 要在 server 公網開放的端口
	ConnID     uint64 `json:"conn_id,omitempty"`     // 訪客連線的識別碼
	Error      string `json:"error,omitempty"`       // 錯誤訊息
}

const maxMsgSize = 1 << 20 // 1 MiB，避免惡意超大長度前綴

// WriteMsg 把一條訊息以「4 bytes 長度前綴 + JSON」的格式寫入 w。
func WriteMsg(w io.Writer, m *Msg) error {
	payload, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(payload) > maxMsgSize {
		return fmt.Errorf("訊息過大: %d bytes", len(payload))
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

// ReadMsg 從 r 讀回一條由 WriteMsg 寫入的訊息。
func ReadMsg(r io.Reader) (*Msg, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(header[:])
	if n > maxMsgSize {
		return nil, fmt.Errorf("訊息過大: %d bytes", n)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	var m Msg
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
