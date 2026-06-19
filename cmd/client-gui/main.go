// client-gui：client 端的原生視窗介面。
//
// 填入 server 位址、密鑰、本機服務與遠端端口，按「啟動」即建立隧道；
// 下方即時顯示日誌。關閉視窗或按「停止」即中止。
package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"reverse-tunnel/internal/tunnelclient"
)

func main() {
	a := app.New()
	w := a.NewWindow("反向隧道 — Client")
	w.Resize(fyne.NewSize(560, 480))

	serverEntry := widget.NewEntry()
	serverEntry.SetPlaceHolder("example.com:7000")
	tokenEntry := widget.NewPasswordEntry()
	tokenEntry.SetPlaceHolder("與 server 相同的密鑰")
	localEntry := widget.NewEntry()
	localEntry.SetText("127.0.0.1:25565")
	remoteEntry := widget.NewEntry()
	remoteEntry.SetText("25565")

	logBox := widget.NewMultiLineEntry()
	logBox.Wrapping = fyne.TextWrapWord
	logBox.Disable() // 唯讀，但仍可捲動與選取

	status := widget.NewLabel("狀態：未啟動")

	lg := newLogger(logBox)

	var (
		mu     sync.Mutex
		cancel context.CancelFunc
	)
	startBtn := widget.NewButton("啟動", nil)
	stopBtn := widget.NewButton("停止", nil)
	stopBtn.Disable()

	setRunning := func(running bool) {
		fyne.Do(func() {
			startBtn.Disable()
			stopBtn.Disable()
			if running {
				stopBtn.Enable()
				status.SetText("狀態：運行中")
			} else {
				startBtn.Enable()
				status.SetText("狀態：已停止")
			}
		})
	}

	startBtn.OnTapped = func() {
		port, err := strconv.Atoi(strings.TrimSpace(remoteEntry.Text))
		if err != nil || port < 1 || port > 65535 {
			lg.logf("遠端端口無效：%s", remoteEntry.Text)
			return
		}
		if strings.TrimSpace(serverEntry.Text) == "" || tokenEntry.Text == "" {
			lg.logf("請填寫 server 位址與密鑰")
			return
		}

		ctx, c := context.WithCancel(context.Background())
		mu.Lock()
		cancel = c
		mu.Unlock()
		setRunning(true)

		client := &tunnelclient.Client{
			Server:     strings.TrimSpace(serverEntry.Text),
			Token:      tokenEntry.Text,
			Local:      strings.TrimSpace(localEntry.Text),
			RemotePort: port,
			Logf:       lg.logf,
		}
		go func() {
			if err := client.Run(ctx); err != nil {
				lg.logf("錯誤：%v", err)
			}
			setRunning(false)
		}()
	}

	stopBtn.OnTapped = func() {
		mu.Lock()
		if cancel != nil {
			cancel()
		}
		mu.Unlock()
	}

	w.SetOnClosed(func() {
		mu.Lock()
		if cancel != nil {
			cancel()
		}
		mu.Unlock()
	})

	form := widget.NewForm(
		widget.NewFormItem("Server 位址", serverEntry),
		widget.NewFormItem("密鑰 Token", tokenEntry),
		widget.NewFormItem("本機服務", localEntry),
		widget.NewFormItem("遠端端口", remoteEntry),
	)
	buttons := container.NewGridWithColumns(2, startBtn, stopBtn)
	top := container.NewVBox(form, buttons, status, widget.NewLabel("日誌："))
	w.SetContent(container.NewBorder(top, nil, nil, nil, container.NewScroll(logBox)))

	w.ShowAndRun()
}

// logger 把日誌訊息（可能來自背景 goroutine）安全地寫進視窗的文字框。
type logger struct {
	box *widget.Entry
	mu  sync.Mutex
	buf strings.Builder
}

func newLogger(box *widget.Entry) *logger { return &logger{box: box} }

func (l *logger) logf(format string, a ...any) {
	line := fmt.Sprintf("%s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...))
	l.mu.Lock()
	l.buf.WriteString(line)
	text := l.buf.String()
	l.mu.Unlock()
	fyne.Do(func() {
		l.box.SetText(text)
		l.box.CursorRow = strings.Count(text, "\n")
	})
}
