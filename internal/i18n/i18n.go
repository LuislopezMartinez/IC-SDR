package i18n

import (
	"encoding/json"
	"fmt"
	"go-zero/contenidos"
	"go-zero/internal/resources"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type Language struct {
	ID    string            `json:"id"`
	Name  string            `json:"name"`
	Flag  string            `json:"flag"`
	Texts map[string]string `json:"texts"`
}
type template struct {
	pattern *regexp.Regexp
	target  string
}

var mu sync.RWMutex
var languages = map[string]Language{}
var spanish map[string]string
var selected = "es"
var preferencePath string
var prefixes [][2]string
var exact map[string]string
var templates []template
var cache = map[string]string{}
var placeholder = regexp.MustCompile(`%%|%([-+#0 ']*[0-9]*(?:\.[0-9]+)?[a-zA-Z])`)

func init() {
	for _, id := range []string{"es", "en"} {
		data, err := contenidos.Defaults.ReadFile(id + ".json")
		if err == nil {
			var l Language
			if json.Unmarshal(data, &l) == nil {
				languages[l.ID] = l
			}
		}
	}
	spanish = languages["es"].Texts
	rebuild()
}
func Source(key string) string {
	mu.RLock()
	defer mu.RUnlock()
	if s, ok := spanish[key]; ok {
		return s
	}
	return key
}
func Directory() string { return filepath.Join(filepath.Dir(resources.WritablePath()), "contenidos") }
func Init() []string    { return Load(Directory(), resources.WritablePath("config", "language.json")) }
func Load(dir, preference string) []string {
	mu.Lock()
	defer mu.Unlock()
	var issues []string
	preferencePath = preference
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, path := range files {
		data, err := os.ReadFile(path)
		var l Language
		if err != nil || json.Unmarshal(data, &l) != nil || l.ID == "" || l.Name == "" || l.Texts == nil {
			issues = append(issues, filepath.Base(path)+": invalid language file")
			continue
		}
		valid := true
		for key, value := range l.Texts {
			if source, ok := spanish[key]; ok && !samePlaceholders(source, value) {
				valid = false
				issues = append(issues, filepath.Base(path)+": invalid placeholders in "+key)
				break
			}
		}
		if valid {
			languages[l.ID] = l
		}
	}
	var pref struct {
		ID string `json:"id"`
	}
	if data, err := os.ReadFile(preference); err == nil && json.Unmarshal(data, &pref) == nil {
		if _, ok := languages[pref.ID]; ok {
			selected = pref.ID
		}
	}
	rebuild()
	return issues
}
func samePlaceholders(a, b string) bool {
	return fmt.Sprint(placeholder.FindAllString(a, -1)) == fmt.Sprint(placeholder.FindAllString(b, -1))
}
func Languages() []Language {
	mu.RLock()
	defer mu.RUnlock()
	var out []Language
	for _, l := range languages {
		out = append(out, Language{ID: l.ID, Name: l.Name, Flag: l.Flag})
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].ID == "es") != (out[j].ID == "es") {
			return out[i].ID == "es"
		}
		return out[i].Name < out[j].Name
	})
	return out
}
func Current() string { mu.RLock(); defer mu.RUnlock(); return selected }

// Codepoints includes letters used by community catalogs before fonts are loaded.
func Codepoints() []rune {
	mu.RLock()
	defer mu.RUnlock()
	seen := map[rune]bool{}
	for _, l := range languages {
		for _, text := range l.Texts {
			for _, r := range text {
				if r >= 32 {
					seen[r] = true
				}
			}
		}
		for _, r := range l.Name {
			if r >= 32 {
				seen[r] = true
			}
		}
	}
	var out []rune
	for r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
func Select(id string) error {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := languages[id]; !ok {
		return fmt.Errorf("unknown language: %s", id)
	}
	path := preferencePath
	if path == "" {
		path = resources.WritablePath("config", "language.json")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]string{"id": id})
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	selected = id
	rebuild()
	return nil
}
func rebuild() {
	exact = map[string]string{}
	templates = nil
	prefixes = nil
	cache = map[string]string{}
	l := languages[selected]
	for key, source := range spanish {
		target := source
		if value := languages["es"].Texts[key]; value != "" {
			target = value
		}
		if value, ok := l.Texts[key]; ok && value != "" {
			target = value
		}
		exact[source] = target
		if source == target {
			continue
		}
		loc := placeholder.FindAllStringIndex(source, -1)
		if len(loc) == 0 {
			if len(source) >= 4 && strings.HasSuffix(source, " ") {
				prefixes = append(prefixes, [2]string{source, target})
			}
			continue
		}
		pattern := "^"
		end := 0
		for _, p := range loc {
			pattern += regexp.QuoteMeta(source[end:p[0]])
			if source[p[0]:p[1]] == "%%" {
				pattern += "%"
			} else {
				pattern += "(.*?)"
			}
			end = p[1]
		}
		pattern += regexp.QuoteMeta(source[end:]) + "$"
		if len(strings.TrimSpace(placeholder.ReplaceAllString(source, ""))) < 3 {
			continue
		}
		templates = append(templates, template{regexp.MustCompile(pattern), target})
	}
	sort.Slice(templates, func(i, j int) bool { return len(templates[i].pattern.String()) > len(templates[j].pattern.String()) })
	sort.Slice(prefixes, func(i, j int) bool { return len(prefixes[i][0]) > len(prefixes[j][0]) })
}

// Display translates presentation text; receiver data and semantic control values stay unchanged.
func Display(text string) string {
	mu.RLock()
	if value, ok := exact[text]; ok {
		mu.RUnlock()
		return value
	}
	if value, ok := cache[text]; ok {
		mu.RUnlock()
		return value
	}
	result := text
	for _, t := range templates {
		matches := t.pattern.FindStringSubmatch(text)
		if matches == nil {
			continue
		}
		n := 0
		result = placeholder.ReplaceAllStringFunc(t.target, func(token string) string {
			if token == "%%" {
				return "%"
			}
			n++
			if n < len(matches) {
				if translated, ok := exact[matches[n]]; ok {
					return translated
				}
				return matches[n]
			}
			return ""
		})
		break
	}
	if result == text {
		for _, p := range prefixes {
			if strings.HasPrefix(text, p[0]) {
				result = p[1] + strings.TrimPrefix(text, p[0])
				break
			}
		}
	}
	mu.RUnlock()
	mu.Lock()
	if len(cache) >= 2048 {
		cache = map[string]string{}
	}
	cache[text] = result
	mu.Unlock()
	return result
}
