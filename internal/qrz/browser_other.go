//go:build !windows

package qrz

import "errors"

func startBrowser(string, chan pageSnapshot) (browserSession, error) {
	return nil, errors.New("QRZ integrado requiere WebView2 en Windows")
}
