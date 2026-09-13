package screens

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const publicIPLookupURL = "https://api.ipify.org"

type publicIPResult struct{ ip string }

func lookupPublicIPv4() (string, error) {
	client := &http.Client{Timeout: 4 * time.Second}
	return lookupPublicIPv4From(context.Background(), client, publicIPLookupURL)
}

func lookupPublicIPv4From(ctx context.Context, client *http.Client, endpoint string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "text/plain")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", errors.New("servicio de IP pública no disponible")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 64))
	if err != nil {
		return "", err
	}
	address := net.ParseIP(strings.TrimSpace(string(data))).To4()
	if address == nil || !address.IsGlobalUnicast() || address.IsPrivate() || address[0] == 100 && address[1]&0xc0 == 64 {
		return "", errors.New("el servicio no devolvió una IPv4 pública")
	}
	return address.String(), nil
}
