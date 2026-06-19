// server CLI：跑在有公網域名的伺服器上。
//
// 用法範例：
//   server -bind :7000 -token mysecret
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"reverse-tunnel/internal/tunnelserver"
)

func main() {
	bind := flag.String("bind", ":7000", "控制/資料連線的監聽位址")
	token := flag.String("token", "", "共享密鑰，client 必須提供相同值（必填）")
	flag.Parse()

	if *token == "" {
		log.Fatal("必須用 -token 設定一個密鑰")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s := &tunnelserver.Server{
		Bind:  *bind,
		Token: *token,
		Logf:  log.Printf,
	}
	if err := s.ListenAndServe(ctx); err != nil {
		log.Fatal(err)
	}
}
