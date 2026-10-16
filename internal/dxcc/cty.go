// Package dxcc reads the CTY.DAT prefix database used by amateur-radio
// logging software. It intentionally keeps the parser independent from the
// UI so other decoders can reuse the same country resolution later.
package dxcc

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// bundledCTY guarantees offline country lookup in portable distributions.
// An external CTY file may still override it to permit database updates.
//
//go:embed data/cty.dat
var bundledCTY string

type Entity struct {
	Name          string
	Continent     string
	PrimaryPrefix string
	CQZone        int
	ITUZone       int
	Latitude      float64
	Longitude     float64
}

type prefixEntry struct {
	prefix string
	entity Entity
}

type Database struct {
	exact    map[string]Entity
	prefixes []prefixEntry
}

func Load(path string) (*Database, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

func LoadBundled() (*Database, error) {
	return Parse(strings.NewReader(bundledCTY))
}

func Parse(r io.Reader) (*Database, error) {
	db := &Database{exact: make(map[string]Entity)}
	scanner := bufio.NewScanner(r)
	// Big CTY contains long exception lists. Leave ample room for future files.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var current Entity
	entities := 0
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimSuffix(scanner.Text(), "\r"))
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, ":") {
			parsed, err := parseHeader(line)
			if err != nil {
				return nil, err
			}
			current = parsed
			entities++
			continue
		}
		if current.Name == "" {
			return nil, fmt.Errorf("cty.dat: prefix list before entity header")
		}
		for _, raw := range strings.Split(strings.TrimSuffix(line, ";"), ",") {
			token := strings.TrimSpace(raw)
			if token == "" {
				continue
			}
			exact := strings.HasPrefix(token, "=")
			token = strings.TrimPrefix(token, "=")
			if cut := strings.IndexAny(token, "([<{~"); cut >= 0 {
				token = token[:cut]
			}
			token = normalize(token)
			if token == "" {
				continue
			}
			if exact {
				db.exact[token] = current
			} else {
				db.prefixes = append(db.prefixes, prefixEntry{prefix: token, entity: current})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if entities == 0 || len(db.prefixes) == 0 {
		return nil, fmt.Errorf("cty.dat: no entities or prefixes found")
	}
	sort.SliceStable(db.prefixes, func(i, j int) bool {
		return len(db.prefixes[i].prefix) > len(db.prefixes[j].prefix)
	})
	return db, nil
}

func parseHeader(line string) (Entity, error) {
	parts := strings.Split(line, ":")
	if len(parts) < 8 {
		return Entity{}, fmt.Errorf("cty.dat: malformed entity header %q", line)
	}
	entity := Entity{
		Name:          strings.TrimSpace(parts[0]),
		Continent:     strings.TrimSpace(parts[3]),
		PrimaryPrefix: normalize(strings.TrimSpace(parts[7])),
	}
	entity.CQZone, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	entity.ITUZone, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
	entity.Latitude, _ = strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
	// CTY stores longitude with the opposite sign to the usual east-positive
	// convention. Convert it once here for map consumers.
	longitude, _ := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
	entity.Longitude = -longitude
	return entity, nil
}

func normalize(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

// Lookup resolves exact CTY exceptions first and then the longest prefix.
// Slash-separated portable calls are also considered, excluding operational
// suffixes which are not locations.
func (db *Database) Lookup(call string) (Entity, bool) {
	if db == nil {
		return Entity{}, false
	}
	call = normalize(call)
	if call == "" {
		return Entity{}, false
	}
	if entity, ok := db.exact[call]; ok {
		return entity, true
	}
	candidates := []string{call}
	for _, part := range strings.Split(call, "/") {
		if part != "" && !portableSuffix(part) {
			candidates = append(candidates, part)
		}
	}
	bestLength := -1
	var best Entity
	for _, candidate := range candidates {
		if entity, ok := db.exact[candidate]; ok && len(candidate) > bestLength {
			best, bestLength = entity, len(candidate)
		}
		for _, entry := range db.prefixes {
			if len(entry.prefix) <= bestLength {
				break
			}
			if strings.HasPrefix(candidate, entry.prefix) {
				best, bestLength = entry.entity, len(entry.prefix)
				break
			}
		}
	}
	return best, bestLength >= 0
}

func portableSuffix(value string) bool {
	switch value {
	case "P", "M", "MM", "AM", "QRP", "QRPP":
		return true
	default:
		return false
	}
}
