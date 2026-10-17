//go:build ignore

// Generates the DXCC entity name to ISO alpha-2 flag mapping embedded by the
// FT8 viewer. Run from the repository root with:
//
//	go run scripts/generate_dxcc_flags.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const source = "https://raw.githubusercontent.com/k0swe/dxcc-json/main/dxcc.json"

type document struct {
	DXCC []struct {
		Name    string `json:"name"`
		Flag    string `json:"flag"`
		Code    string `json:"countryCode"`
		Prefix  string `json:"prefix"`
		Deleted bool   `json:"deleted"`
	} `json:"dxcc"`
}

func main() {
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Get(source)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		panic(response.Status)
	}
	var input document
	if err = json.NewDecoder(response.Body).Decode(&input); err != nil {
		panic(err)
	}
	output := make(map[string]string)
	byPrefix := make(map[string]string)
	for _, entity := range input.DXCC {
		if entity.Deleted {
			continue
		}
		code, _ := flagAlpha2(entity.Flag)
		if len(code) != 2 {
			code = strings.ToUpper(strings.TrimSpace(entity.Code))
		}
		if len(code) == 2 {
			output[entity.Name] = code
			byPrefix[normalizePrefix(entity.Prefix)] = code
		}
	}
	// Big CTY sometimes uses a different display name for the same DXCC entity.
	// Its primary prefix is stable, so add those names automatically instead of
	// maintaining an ever-growing hand-written alias list.
	file, err := os.Open("internal/dxcc/data/cty.dat")
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasSuffix(line, ":") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 8 {
			continue
		}
		name, prefix := strings.TrimSpace(parts[0]), normalizePrefix(parts[7])
		if output[name] == "" && byPrefix[prefix] != "" {
			output[name] = byPrefix[prefix]
		}
	}
	file.Close()
	if err = scanner.Err(); err != nil {
		panic(err)
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		panic(err)
	}
	data = append(data, '\n')
	if err = os.WriteFile("screens/assets/flags/dxcc-flags.json", data, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("generated mappings for %d active DXCC entities\n", len(output))
}

func normalizePrefix(value string) string {
	return strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(value), "*"))
}

func flagAlpha2(flag string) (string, bool) {
	runes := []rune(flag)
	if len(runes) != 2 || runes[0] < 0x1F1E6 || runes[0] > 0x1F1FF || runes[1] < 0x1F1E6 || runes[1] > 0x1F1FF {
		return "", false
	}
	return string([]byte{byte('A' + runes[0] - 0x1F1E6), byte('A' + runes[1] - 0x1F1E6)}), true
}
