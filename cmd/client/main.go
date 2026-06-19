// client CLI：跑在你自己那台位於 NAT/防火牆後面的機器上。
//
// 用法範例（把本機 25565 的 Minecraft 透過 example.com:7000 開放到公網 25565）：
//   client -server example.com:7000 -token mysecret -local 127.0.0.1:25565 -remote-port 25565
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"reverse-tunnel/internal/tunnelclient"
)

func main() {
	serverAddr := flag.String("server", "", "server 的位址，例如 example.com:7000（必填）")
	token := flag.String("token", "", "與 server 相同的共享密鑰（必填）")
	local := flag.String("local", "127.0.0.1:25565", "要被穿透的本機服務位址")
	remotePort := flag.Int("remote-port", 25565, "要在 server 公網開放的端口")
	flag.Parse()

	if *serverAddr == "" || *token == "" {
		log.Fatal("必須用 -server 與 -token 指定 server 位址與密鑰")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c := &tunnelclient.Client{
		Server:     *serverAddr,
		Token:      *token,
		Local:      *local,
		RemotePort: *remotePort,
		Logf:       log.Printf,
	}
	if err := c.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
