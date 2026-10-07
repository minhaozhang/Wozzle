//go:build windows

// Package desktop provides the Windows shell integration: system tray,
// WebView2 host window, boot autostart and console attachment.
package desktop

import (
	_ "embed"

	"github.com/getlantern/systray"
)

//go:embed wozzle.ico
var iconBytes []byte

// TrayHandlers wires tray menu actions to the app.
type TrayHandlers struct {
	OnOpen    func() // 打开面板
	OnBrowser func() // 在浏览器中打开
	OnQuit    func() // 退出
}

// StartTray spawns the tray icon on its own goroutine. Menu methods in
// getlantern/systray are safe from any goroutine on Windows.
func StartTray(h TrayHandlers, autoStartState func() bool, setAutoStart func(bool) error) <-chan struct{} {
	started := make(chan struct{})
	go systray.Run(func() {
		systray.SetIcon(iconBytes)
		systray.SetTooltip("Wozzle — WSL 容器监控")

		mOpen := systray.AddMenuItem("打开面板", "打开 Wozzle 窗口")
		mBrowser := systray.AddMenuItem("在浏览器中打开", "用默认浏览器打开")
		systray.AddSeparator()
		mAuto := systray.AddMenuItemCheckbox("开机自启", "登录 Windows 后自动启动", autoStartState())
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("退出", "退出 Wozzle")

		go func() {
			for range mOpen.ClickedCh {
				h.OnOpen()
			}
		}()
		go func() {
			for range mBrowser.ClickedCh {
				h.OnBrowser()
			}
		}()
		go func() {
			for range mAuto.ClickedCh {
				on := !mAuto.Checked()
				if err := setAutoStart(on); err == nil {
					if on {
						mAuto.Check()
					} else {
						mAuto.Uncheck()
					}
				}
			}
		}()
		go func() {
			for range mQuit.ClickedCh {
				h.OnQuit()
				systray.Quit()
			}
		}()

		close(started)
	}, nil)
	return started
}
