package qrz

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/biter777/countries"
)

type Result struct {
	Callsign, Country, Name, City, Region, PostalCode string
	Member, Lookups, Locator, ImageURL, ImagePath     string
	CountryCode, Continent, FlagPath                  string
	Latitude, Longitude                               float64
	Coordinates                                       bool
	Authenticated                                     bool
	AuthenticationRequired                            bool
	Err                                               error
}

type Client struct {
	profileDir, cacheDir string
	mu                   sync.Mutex
	browser              browserSession
	snapshots            chan pageSnapshot
	generation           uint64
	cancel               chan struct{}
	results              chan Result
}

type browserSession interface {
	Navigate(string) error
	Alive() bool
	Hide()
	Show()
	Close()
}

func New(profileDir, cacheDir string) *Client {
	return &Client{profileDir: profileDir, cacheDir: cacheDir, snapshots: make(chan pageSnapshot, 2), results: make(chan Result, 4)}
}

func (c *Client) Results() <-chan Result { return c.results }

func (c *Client) Open() { c.startLookup("") }

// ShowProfile opens the requested station profile in the persistent,
// integrated QRZ browser. It is deliberately user initiated: normal lookups
// keep the browser hidden once the session has been authenticated.
func (c *Client) ShowProfile(callsign string) error {
	callsign = strings.ToUpper(strings.TrimSpace(callsign))
	if !regexp.MustCompile(`^[A-Z0-9/]{3,16}$`).MatchString(callsign) {
		return errors.New("indicativo no válido")
	}
	if err := c.navigate("https://www.qrz.com/db/" + url.PathEscape(callsign)); err != nil {
		return err
	}
	c.mu.Lock()
	browser := c.browser
	c.mu.Unlock()
	if browser != nil {
		browser.Show()
	}
	return nil
}

// HideProfile hides the integrated browser without closing it, preserving the
// authenticated QRZ session for later lookups or profile viewing.
func (c *Client) HideProfile() {
	c.hideBrowser()
}

func (c *Client) Lookup(callsign string) {
	callsign = strings.ToUpper(strings.TrimSpace(callsign))
	if !regexp.MustCompile(`^[A-Z0-9/]{3,16}$`).MatchString(callsign) {
		c.publish(Result{Err: errors.New("indicativo no válido")})
		return
	}
	c.startLookup(callsign)
}

func (c *Client) startLookup(callsign string) {
	c.mu.Lock()
	if c.cancel != nil {
		close(c.cancel)
	}
	c.cancel = make(chan struct{})
	cancel := c.cancel
	c.generation++
	generation := c.generation
	c.mu.Unlock()
	go func() {
		pageURL := "https://www.qrz.com/"
		if callsign != "" {
			pageURL = "https://www.qrz.com/db/" + url.PathEscape(callsign)
		}
		if err := c.navigate(pageURL); err != nil {
			c.publish(Result{Callsign: callsign, Err: err})
			return
		}
		timer := time.NewTimer(4 * time.Minute)
		defer timer.Stop()
		var readySince time.Time
		var pending Result
		loginRequested := false
		for {
			select {
			case <-cancel:
				return
			case snapshot := <-c.snapshots:
				if !c.current(generation) {
					return
				}
				if !snapshot.Authenticated {
					if !loginRequested {
						loginRequested = true
						c.showBrowser()
						c.publish(Result{Callsign: callsign, AuthenticationRequired: true})
					}
					continue
				}
				if callsign == "" {
					c.hideBrowser()
					c.publish(Result{Authenticated: true})
					return
				}
				result, ready := extractSnapshot(callsign, snapshot)
				if !ready {
					continue
				}
				pending = result
				if result.ImageURL != "" {
					pending.ImagePath, _ = c.downloadImage(result.ImageURL, callsign)
				}
				if pending.ImagePath == "" {
					pending.ImagePath = c.cachedImage(callsign)
				}
				if pending.CountryCode != "" {
					pending.FlagPath, _ = c.countryFlag(pending.CountryCode)
				}
				if pending.ImagePath != "" {
					pending.Authenticated = true
					c.hideBrowser()
					c.publish(pending)
					return
				}
				if readySince.IsZero() {
					readySince = time.Now()
					continue
				}
				// QRZ often paints the header before the profile photograph.
				// Keep consuming WebView snapshots briefly so the late image URL
				// is included without delaying text-only profiles excessively.
				if time.Since(readySince) >= 5*time.Second {
					pending.Authenticated = true
					c.hideBrowser()
					c.publish(pending)
					return
				}
			case <-timer.C:
				if c.current(generation) {
					c.publish(Result{Callsign: callsign, Err: errors.New("QRZ no devolvió una ficha; revise el login en la ventana integrada")})
				}
				return
			}
		}
	}()
}

func (c *Client) hideBrowser() {
	c.mu.Lock()
	browser := c.browser
	c.mu.Unlock()
	if browser != nil {
		browser.Hide()
	}
}

func (c *Client) showBrowser() {
	c.mu.Lock()
	browser := c.browser
	c.mu.Unlock()
	if browser != nil {
		browser.Show()
	}
}

func (c *Client) Logout() error {
	c.mu.Lock()
	c.generation++
	if c.cancel != nil {
		close(c.cancel)
		c.cancel = nil
	}
	browser := c.browser
	c.browser = nil
	c.mu.Unlock()
	if browser != nil {
		browser.Close()
	}
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		err = os.RemoveAll(c.profileDir)
		if err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return err
}

func (c *Client) cachedImage(callsign string) string {
	path := filepath.Join(c.cacheDir, strings.ReplaceAll(callsign, "/", "_")+".jpg")
	if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 0 {
		return path
	}
	return ""
}

func (c *Client) current(generation uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generation == generation
}

func (c *Client) publish(result Result) {
	select {
	case c.results <- result:
	default:
		select {
		case <-c.results:
		default:
		}
		c.results <- result
	}
}

func (c *Client) navigate(pageURL string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureBrowserLocked(); err != nil {
		return err
	}
	return c.browser.Navigate(pageURL)
}

func (c *Client) ensureBrowserLocked() error {
	if c.browser != nil && c.browser.Alive() {
		return nil
	}
	if err := os.MkdirAll(c.profileDir, 0700); err != nil {
		return err
	}
	browser, err := startBrowser(c.profileDir, c.snapshots)
	if err == nil {
		c.browser = browser
	}
	return err
}

type pageSnapshot struct {
	URL, Title, Text, ImageURL string
	CountryCode                string `json:"countryCode"`
	Authenticated              bool
}

func extractSnapshot(callsign string, snapshot pageSnapshot) (Result, bool) {
	if !snapshot.Authenticated || !strings.Contains(strings.ToUpper(snapshot.Text), callsign) || strings.Contains(strings.ToLower(snapshot.Title), "just a moment") {
		return Result{}, false
	}
	result := parseSnapshot(callsign, snapshot)
	return result, result.Name != "" || result.City != "" || result.Coordinates
}

func parseSnapshot(callsign string, snapshot pageSnapshot) Result {
	result := Result{Callsign: callsign, ImageURL: snapshot.ImageURL}
	lines := make([]string, 0)
	for _, line := range strings.Split(strings.ReplaceAll(snapshot.Text, "\r", ""), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	index, bestScore, inline := -1, -1, ""
	callPattern := regexp.MustCompile(`(?i)(^|\s)` + regexp.QuoteMeta(callsign) + `(\s|$)`)
	for i, line := range lines {
		location := callPattern.FindStringIndex(line)
		if location == nil {
			continue
		}
		end := min(len(lines), i+14)
		context := strings.ToLower(strings.Join(lines[i:end], "\n"))
		score := 0
		if strings.Contains(context, "email") {
			score += 5
		}
		if strings.Contains(context, "ham member") || strings.Contains(context, "lookups") {
			score += 3
		}
		if score >= bestScore {
			index, bestScore = i, score
			inline = strings.TrimSpace(line[location[1]:])
		}
	}
	if index >= 0 {
		profile := make([]string, 0, 6)
		if inline != "" {
			profile = append(profile, inline)
		}
		for _, line := range lines[index+1 : min(len(lines), index+14)] {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "email") || strings.Contains(lower, "ham member") || strings.Contains(lower, "lookups") {
				break
			}
			if strings.Contains(lower, "qrz/30") || strings.EqualFold(line, callsign) {
				continue
			}
			profile = append(profile, line)
		}
		if len(profile) >= 2 && strings.EqualFold(profile[0], profile[len(profile)-1]) {
			profile = profile[1:]
		}
		if len(profile) > 0 {
			result.Country = profile[len(profile)-1]
			profile = profile[:len(profile)-1]
		}
		if len(profile) > 0 {
			result.Name = profile[0]
		}
		if len(profile) > 1 {
			result.City = profile[1]
		}
		if len(profile) > 2 {
			result.Region = strings.Join(profile[2:], " · ")
		}
	}
	if regexp.MustCompile(`(?i)ham member`).MatchString(snapshot.Text) {
		result.Member = "HAM MEMBER"
	}
	result.Country = normalizeCountryName(result.Country)
	if country := countryFromSnapshot(result.Country, snapshot.CountryCode); country != countries.Unknown {
		result.CountryCode = country.Alpha2()
		result.Continent = spanishContinent(country.Region().String())
	}
	if result.Lookups == "" {
		if match := regexp.MustCompile(`(?i)Lookups?\s*[: ]\s*([0-9.,]+)`).FindStringSubmatch(snapshot.Text); len(match) > 1 {
			result.Lookups = match[1]
		}
	}
	if match := regexp.MustCompile(`(?i)(?:Grid Square|Grid|Locator)\s*[:#-]?\s*([A-R]{2}[0-9]{2}[A-X]{0,2})`).FindStringSubmatch(snapshot.Text); len(match) > 1 {
		result.Locator = match[1]
	}
	latMatch := regexp.MustCompile(`(?i)Latitude\s+(-?[0-9]+(?:[.,][0-9]+)?)`).FindStringSubmatch(snapshot.Text)
	lonMatch := regexp.MustCompile(`(?i)Longitude\s+(-?[0-9]+(?:[.,][0-9]+)?)`).FindStringSubmatch(snapshot.Text)
	if len(latMatch) > 1 && len(lonMatch) > 1 {
		result.Latitude, _ = strconv.ParseFloat(strings.ReplaceAll(latMatch[1], ",", "."), 64)
		result.Longitude, _ = strconv.ParseFloat(strings.ReplaceAll(lonMatch[1], ",", "."), 64)
		result.Coordinates = result.Latitude >= -90 && result.Latitude <= 90 && result.Longitude >= -180 && result.Longitude <= 180
	}
	return result
}

// normalizeCountryName removes decorative flag glyphs and punctuation that
// QRZ may place around the visible country name. Regional-indicator emoji are
// symbols rather than letters, so they must not participate in ISO lookup.
func normalizeCountryName(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	value = strings.TrimFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	value = strings.TrimSpace(value)
	if match := regexp.MustCompile(`(?i)^(?:country|nation|pa[ií]s)\s*[:\-]\s*(.+)$`).FindStringSubmatch(value); len(match) == 2 {
		value = strings.TrimSpace(match[1])
	}
	return value
}

func countryFromSnapshot(name, alpha2 string) countries.CountryCode {
	name = normalizeCountryName(name)
	if country := countryFromNameParts(name); country != countries.Unknown {
		return country
	}
	alpha2 = strings.ToUpper(strings.TrimSpace(alpha2))
	if regexp.MustCompile(`^[A-Z]{2}$`).MatchString(alpha2) {
		if country := countries.ByName(alpha2); country != countries.Unknown {
			return country
		}
	}
	return countries.Unknown
}

func countryFromNameParts(name string) countries.CountryCode {
	if country := countries.ByName(name); country != countries.Unknown {
		return country
	}
	words := strings.Fields(name)
	for length := len(words) - 1; length > 0; length-- {
		for start := 0; start+length <= len(words); start++ {
			candidate := strings.Join(words[start:start+length], " ")
			// Do not interpret incidental two-letter words such as "is" in a
			// login warning as ISO country codes (IS is Iceland).
			if length == 1 && len([]rune(candidate)) < 4 {
				continue
			}
			if country := countries.ByName(candidate); country != countries.Unknown {
				return country
			}
		}
	}
	return countries.Unknown
}

func spanishContinent(value string) string {
	switch value {
	case "Africa":
		return "África"
	case "Europe":
		return "Europa"
	case "North America":
		return "América del Norte"
	case "South America":
		return "América del Sur"
	case "Oceania":
		return "Oceanía"
	case "Antarctica":
		return "Antártida"
	default:
		return value
	}
}

func (c *Client) countryFlag(alpha2 string) (string, error) {
	alpha2 = strings.ToUpper(strings.TrimSpace(alpha2))
	path := filepath.Join(c.cacheDir, "flag-"+alpha2+".png")
	if data, err := os.ReadFile(path); err == nil && validPNG(data) {
		return path, nil
	}
	_ = os.Remove(path)
	sources := []struct {
		address string
		referer string
	}{
		{"https://static.qrz.com/static/flags-iso/flat/32/" + url.PathEscape(alpha2) + ".png", "https://www.qrz.com/"},
		{"https://flagcdn.com/w40/" + strings.ToLower(url.PathEscape(alpha2)) + ".png", ""},
	}
	var lastErr error
	var data []byte
	for _, source := range sources {
		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, source.address, nil)
		if err != nil {
			lastErr = err
			continue
		}
		request.Header.Set("User-Agent", "Mozilla/5.0")
		if source.referer != "" {
			request.Header.Set("Referer", source.referer)
		}
		response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
		if err != nil {
			lastErr = err
			continue
		}
		candidate, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if response.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("bandera: HTTP %d", response.StatusCode)
			continue
		}
		if !validPNG(candidate) {
			lastErr = errors.New("la respuesta de bandera no es una imagen PNG válida")
			continue
		}
		data = candidate
		break
	}
	if len(data) == 0 {
		return "", lastErr
	}
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0600)
}

func validPNG(data []byte) bool {
	return len(data) >= 24 && string(data[:8]) == "\x89PNG\r\n\x1a\n"
}

func (c *Client) downloadImage(source, callsign string) (string, error) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, source, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("Referer", "https://www.qrz.com/db/"+callsign)
	response, err := (&http.Client{Timeout: 12 * time.Second}).Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("imagen QRZ: HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil || len(data) == 0 {
		return "", err
	}
	if err = os.MkdirAll(c.cacheDir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(c.cacheDir, strings.ReplaceAll(callsign, "/", "_")+".jpg")
	return path, os.WriteFile(path, data, 0600)
}

func (c *Client) Close() {
	c.mu.Lock()
	c.generation++
	if c.cancel != nil {
		close(c.cancel)
		c.cancel = nil
	}
	browser := c.browser
	c.browser = nil
	c.mu.Unlock()
	if browser != nil {
		browser.Close()
	}
}
