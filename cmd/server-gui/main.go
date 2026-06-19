// server-gui：server 端的原生視窗介面。
//
// 填入監聽位址與密鑰，按「啟動」即開始接受 client 連線；
// 下方即時顯示連線狀態與日誌。
package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"reverse-tunnel/internal/tunnelserver"
)

func main() {
	a := app.New()
	w := a.NewWindow("反向隧道 — Server")
	w.Resize(fyne.NewSize(560, 460))

	bindEntry := widget.NewEntry()
	bindEntry.SetText(":7000")
	tokenEntry := widget.NewPasswordEntry()
	tokenEntry.SetPlaceHolder("設定一個夠長的密鑰")

	logBox := widget.NewMultiLineEntry()
	logBox.Wrapping = fyne.TextWrapWord
	logBox.Disable()

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
		if tokenEntry.Text == "" {
			lg.logf("請設定密鑰")
			return
		}
		ctx, c := context.WithCancel(context.Background())
		mu.Lock()
		cancel = c
		mu.Unlock()
		setRunning(true)

		srv := &tunnelserver.Server{
			Bind:  strings.TrimSpace(bindEntry.Text),
			Token: tokenEntry.Text,
			Logf:  lg.logf,
		}
		go func() {
			if err := srv.ListenAndServe(ctx); err != nil {
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
		widget.NewFormItem("監聽位址", bindEntry),
		widget.NewFormItem("密鑰 Token", tokenEntry),
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
