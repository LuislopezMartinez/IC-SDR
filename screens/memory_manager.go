package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-zero/simpleui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type memoryManagerProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
}
type memoryManagerCommand struct {
	Action string      `json:"action"`
	ID     int64       `json:"id"`
	Memory MemoryEntry `json:"memory"`
}

type memoryManager struct {
	path, groupsPath, commandPath      string
	memories                           []MemoryEntry
	catalog                            []MemoryCatalogEntry
	groupColors                        map[string]string
	groups                             []string
	group, search                      string
	selected, scroll, groupScroll, tab int
	catalogSelected, catalogScroll     int
	nextRead                           time.Time
	stamp, groupsStamp                 time.Time
	searchActive                       bool
	modal                              string
	edit                               MemoryEntry
	editIndex, field, color            int
	buffer, errorText                  string
}

var managerNavy = rl.Color{R: 10, G: 35, B: 61, A: 255}
var managerBlue = rl.Color{R: 22, G: 120, B: 168, A: 255}
var managerCyan = rl.Color{R: 23, G: 169, B: 205, A: 255}
var managerGreen = rl.Color{R: 35, G: 170, B: 105, A: 255}
var managerOrange = rl.Color{R: 236, G: 151, B: 30, A: 255}
var managerRed = rl.Color{R: 203, G: 70, B: 74, A: 255}
var managerInk = rl.Color{R: 22, G: 48, B: 70, A: 255}
var managerMuted = rl.Color{R: 101, G: 125, B: 144, A: 255}
var managerLine = rl.Color{R: 197, G: 216, B: 228, A: 255}
var managerPaper = rl.Color{R: 243, G: 249, B: 252, A: 255}
var managerPanel = rl.Color{R: 231, G: 243, B: 249, A: 255}

func RunMemoryManager(path, groupsPath, commandPath, selectedGroup string) {
	simpleui.SetMode(1600, 900, simpleui.Stretch)
	simpleui.SetInitialWindowSize(1560, 860)
	simpleui.SetMinimumSize(1100, 650)
	simpleui.SetCanvasFilter(rl.FilterBilinear)
	simpleui.SetTextScale(1.15)
	simpleui.SetTitle("IC-SDR · GESTOR DE MEMORIAS")
	simpleui.SetMaximizeKey(0)
	simpleui.SetColors(managerPaper, managerNavy)
	if strings.TrimSpace(selectedGroup) == "" {
		selectedGroup = "TODAS"
	}
	m := &memoryManager{path: path, groupsPath: groupsPath, commandPath: commandPath, selected: -1, catalogSelected: -1, editIndex: -1, groupColors: map[string]string{}, group: selectedGroup, catalog: nativeMemoryCatalog()}
	m.read(true)
	simpleui.Run(m.draw)
}

func (m *memoryManager) read(force bool) {
	if !force && (m.modal != "" || time.Now().Before(m.nextRead)) {
		return
	}
	m.nextRead = time.Now().Add(300 * time.Millisecond)
	if info, err := os.Stat(m.path); err == nil && (force || info.ModTime().After(m.stamp)) {
		var memories []MemoryEntry
		if data, e := os.ReadFile(m.path); e == nil && json.Unmarshal(data, &memories) == nil {
			m.memories, m.stamp = memories, info.ModTime()
		}
	}
	if info, err := os.Stat(m.groupsPath); err == nil && (force || info.ModTime().After(m.groupsStamp)) {
		if data, e := os.ReadFile(m.groupsPath); e == nil {
			_ = json.Unmarshal(data, &m.groupColors)
		}
		m.groupsStamp = info.ModTime()
	}
	m.rebuildGroups()
}

func (m *memoryManager) rebuildGroups() {
	seen := map[string]bool{}
	groups := []string{"TODAS"}
	for group := range m.groupColors {
		if strings.TrimSpace(group) != "" {
			seen[group] = true
			groups = append(groups, group)
		}
	}
	for _, memory := range m.memories {
		group := strings.TrimSpace(memory.Group)
		if group == "" {
			group = "SIN GRUPO"
		}
		if !seen[group] {
			seen[group] = true
			groups = append(groups, group)
		}
	}
	sort.Slice(groups[1:], func(i, j int) bool { return strings.ToLower(groups[i+1]) < strings.ToLower(groups[j+1]) })
	m.groups = groups
	found := m.group == "TODAS"
	for _, group := range groups {
		found = found || group == m.group
	}
	if !found {
		m.group = "TODAS"
	}
}

func (m *memoryManager) save() {
	data, err := json.MarshalIndent(m.memories, "", "  ")
	if err != nil {
		m.errorText = err.Error()
		return
	}
	_ = os.MkdirAll(filepath.Dir(m.path), 0755)
	if err = replaceLiveSnapshot(m.path, data); err != nil {
		m.errorText = err.Error()
		return
	}
	if info, e := os.Stat(m.path); e == nil {
		m.stamp = info.ModTime()
	}
	m.errorText = "Cambios guardados"
}

func (m *memoryManager) saveGroups() {
	data, _ := json.MarshalIndent(m.groupColors, "", "  ")
	_ = os.MkdirAll(filepath.Dir(m.groupsPath), 0755)
	if err := replaceLiveSnapshot(m.groupsPath, data); err != nil {
		m.errorText = err.Error()
	}
	if info, e := os.Stat(m.groupsPath); e == nil {
		m.groupsStamp = info.ModTime()
	}
}

func (m *memoryManager) filtered() []int {
	query := strings.ToLower(strings.TrimSpace(m.search))
	result := make([]int, 0, len(m.memories))
	for i, memory := range m.memories {
		group := strings.TrimSpace(memory.Group)
		if group == "" {
			group = "SIN GRUPO"
		}
		if m.group != "TODAS" && group != m.group {
			continue
		}
		haystack := strings.ToLower(memory.Name + " " + memory.Description + " " + memory.Mode + " " + group + " " + fmt.Sprintf("%.6f", float64(memory.FrequencyHz)/1e6))
		if query == "" || strings.Contains(haystack, query) {
			result = append(result, i)
		}
	}
	return result
}

func (m *memoryManager) draw() {
	m.read(false)
	m.input()
	rl.ClearBackground(managerPaper)
	m.drawHeader()
	if m.tab == 0 {
		m.drawWorkspace()
	} else {
		m.drawCatalogWorkspace()
	}
	m.drawFooter()
	if m.modal != "" {
		m.drawModal()
	}
}

func mmText(text string, x, y float32, size int32, color rl.Color) {
	simpleui.DrawTextStyled(text, x, y, size, simpleui.FontRegular, color)
}
func mmBold(text string, x, y float32, size int32, color rl.Color) {
	simpleui.DrawTextStyled(text, x, y, size, simpleui.FontSemiBold, color)
}
func mmBox(r rl.Rectangle, fill, border rl.Color) {
	rl.DrawRectangleRounded(r, .08, 6, fill)
	rl.DrawRectangleRoundedLinesEx(r, .08, 6, 1, border)
}
func mmInside(p rl.Vector2, r rl.Rectangle) bool {
	return p.X >= r.X && p.X <= r.X+r.Width && p.Y >= r.Y && p.Y <= r.Y+r.Height
}
func mmButton(r rl.Rectangle, label string, fill, text rl.Color) {
	rl.DrawRectangleRounded(r, .14, 7, fill)
	mmBold(label, r.X+14, r.Y+(r.Height-16)/2, 14, text)
}

func (m *memoryManager) drawHeader() {
	rl.DrawRectangle(0, 0, 1600, 72, managerNavy)
	rl.DrawCircle(36, 36, 17, managerCyan)
	mmBold("M", 28, 25, 17, rl.White)
	mmBold("IC-SDR", 65, 17, 21, rl.White)
	mmText("GESTOR DE MEMORIAS", 65, 43, 12, rl.Color{R: 170, G: 211, B: 229, A: 255})
	mmText("Banco compartido con el receptor", 1175, 27, 13, rl.Color{R: 190, G: 219, B: 232, A: 255})
	tabs := []string{"MIS MEMORIAS", "CATÁLOGO OFICIAL", "SWL / ONDA CORTA", "ACTUALIZACIONES"}
	rl.DrawRectangle(0, 72, 1600, 50, rl.White)
	for i, tab := range tabs {
		x := float32(28 + i*218)
		color := managerMuted
		if i == m.tab {
			color = managerBlue
			rl.DrawRectangle(int32(x), 118, 180, 4, managerCyan)
		}
		mmBold(tab, x, 89, 13, color)
	}
	rl.DrawLine(0, 122, 1600, 122, managerLine)
	rl.DrawRectangle(0, 123, 1600, 63, managerPanel)
	search := rl.Rectangle{X: 24, Y: 137, Width: 430, Height: 36}
	mmBox(search, rl.White, managerLine)
	value := m.search
	if value == "" {
		value = "BUSCAR: nombre, frecuencia, modo o grupo..."
	}
	color := managerInk
	if m.search == "" {
		color = managerMuted
	}
	mmText(value, 39, 148, 13, color)
	mmButton(rl.Rectangle{X: 1120, Y: 136, Width: 118, Height: 38}, "IMPORTAR", rl.White, managerBlue)
	mmButton(rl.Rectangle{X: 1248, Y: 136, Width: 112, Height: 38}, "EXPORTAR", rl.White, managerBlue)
	mmButton(rl.Rectangle{X: 1370, Y: 136, Width: 205, Height: 38}, "+ NUEVA MEMORIA", managerGreen, rl.White)
}

func (m *memoryManager) drawWorkspace() {
	left := rl.Rectangle{X: 20, Y: 200, Width: 255, Height: 646}
	center := rl.Rectangle{X: 290, Y: 200, Width: 970, Height: 646}
	right := rl.Rectangle{X: 1275, Y: 200, Width: 305, Height: 646}
	mmBox(left, rl.White, managerLine)
	mmBox(center, rl.White, managerLine)
	mmBox(right, rl.White, managerLine)
	mmBold("COLECCIONES", 40, 220, 13, managerMuted)
	mmButton(rl.Rectangle{X: 38, Y: 252, Width: 219, Height: 37}, "+ NUEVO GRUPO", managerBlue, rl.White)
	start := m.groupScroll
	maxRows := 18
	if start > max(0, len(m.groups)-maxRows) {
		start = max(0, len(m.groups)-maxRows)
		m.groupScroll = start
	}
	for row, group := range m.groups[start:min(len(m.groups), start+maxRows)] {
		y := float32(306 + row*28)
		selected := group == m.group
		if selected {
			rl.DrawRectangleRounded(rl.Rectangle{X: 31, Y: y - 5, Width: 233, Height: 26}, .15, 5, rl.Color{R: 219, G: 241, B: 250, A: 255})
		}
		color := m.groupColor(group)
		if group == "TODAS" {
			color = managerBlue
		}
		rl.DrawCircle(44, int32(y+7), 5, color)
		mmBold(group, 57, y, 13, managerInk)
		count := 0
		for _, memory := range m.memories {
			if group == "TODAS" || memoryGroupPlain(memory) == group {
				count++
			}
		}
		mmText(strconv.Itoa(count), 231, y, 12, managerMuted)
	}
	m.drawTable(center)
	m.drawInspector(right)
}

func (m *memoryManager) drawTable(_ rl.Rectangle) {
	mmBold("MEMORIAS", 310, 220, 13, managerMuted)
	mmText(fmt.Sprintf("%d elementos", len(m.filtered())), 1129, 220, 12, managerMuted)
	rl.DrawRectangle(301, 249, 948, 38, managerPanel)
	headers := []string{"ACT.", "NOMBRE", "FRECUENCIA", "MODO", "FILTRO", "PASO", "GRUPO", "PRI."}
	xs := []float32{311, 356, 592, 724, 800, 878, 956, 1212}
	for i, h := range headers {
		mmBold(h, xs[i], 261, 11, managerMuted)
	}
	indices := m.filtered()
	visible := 20
	m.scroll = min(max(m.scroll, 0), max(0, len(indices)-visible))
	end := min(len(indices), m.scroll+visible)
	for row, index := range indices[m.scroll:end] {
		memory := m.memories[index]
		y := float32(292 + row*26)
		if index == m.selected {
			rl.DrawRectangle(301, int32(y-4), 948, 25, rl.Color{R: 210, G: 238, B: 249, A: 255})
		}
		rl.DrawRectangle(301, int32(y-4), 4, 25, m.groupColor(memoryGroupPlain(memory)))
		active := "--"
		if memory.ScanEnabled {
			active = "ON"
		}
		mmText(active, 314, y, 13, managerCyan)
		mmBold(trimMemory(memory.Name, 24), 356, y, 12, m.groupColor(memoryGroupPlain(memory)))
		mmText(fmt.Sprintf("%.6f", float64(memory.FrequencyHz)/1e6), 592, y, 12, managerInk)
		mmText(memory.Mode, 724, y, 12, managerInk)
		mmText(formatFilterBandwidth(memory.FilterBandwidthHz), 800, y, 12, managerInk)
		mmText(formatStep(memory.StepHz), 878, y, 12, managerInk)
		mmText(trimMemory(memoryGroupPlain(memory), 18), 956, y, 12, managerMuted)
		if memory.Priority {
			mmText("!", 1212, y, 14, managerOrange)
		}
		rl.DrawLine(306, int32(y+20), 1244, int32(y+20), rl.Color{R: 226, G: 237, B: 243, A: 255})
	}
	if len(indices) == 0 {
		mmBold("No hay memorias en esta colección", 555, 480, 17, managerMuted)
	}
}

func (m *memoryManager) drawInspector(_ rl.Rectangle) {
	mmBold("INSPECTOR", 1295, 220, 13, managerMuted)
	if m.selected < 0 || m.selected >= len(m.memories) {
		mmText("Selecciona una memoria", 1310, 275, 14, managerMuted)
		mmText("para ver y editar sus datos.", 1310, 300, 13, managerMuted)
		return
	}
	memory := m.memories[m.selected]
	color := m.groupColor(memoryGroupPlain(memory))
	rl.DrawCircle(1305, 270, 7, color)
	mmBold(trimMemory(memory.Name, 25), 1322, 258, 18, managerInk)
	mmText(memoryGroupPlain(memory), 1322, 282, 12, color)
	mmBold(fmt.Sprintf("%.6f MHz", float64(memory.FrequencyHz)/1e6), 1300, 326, 25, managerBlue)
	mmText(memory.Mode+"   ·   "+formatFilterBandwidth(memory.FilterBandwidthHz)+"   ·   "+formatStep(memory.StepHz), 1300, 362, 13, managerInk)
	rl.DrawLine(1298, 394, 1558, 394, managerLine)
	mmBold("DESCRIPCIÓN", 1300, 414, 11, managerMuted)
	mmText(trimMemory(memory.Description, 34), 1300, 438, 13, managerInk)
	mmButton(rl.Rectangle{X: 1297, Y: 500, Width: 260, Height: 42}, "SINTONIZAR", managerGreen, rl.White)
	mmButton(rl.Rectangle{X: 1297, Y: 552, Width: 126, Height: 38}, "EDITAR", managerBlue, rl.White)
	mmButton(rl.Rectangle{X: 1431, Y: 552, Width: 126, Height: 38}, "DUPLICAR", managerPanel, managerBlue)
	mmButton(rl.Rectangle{X: 1297, Y: 600, Width: 126, Height: 38}, "MOVER", managerPanel, managerOrange)
	mmButton(rl.Rectangle{X: 1431, Y: 600, Width: 126, Height: 38}, "×  ELIMINAR", rl.Color{R: 253, G: 235, B: 235, A: 255}, managerRed)
	status := "DESACTIVADA"
	if memory.ScanEnabled {
		status = "ACTIVA"
	}
	mmBold("ESCANEO  "+status, 1300, 690, 12, managerGreen)
	priority := "NORMAL"
	if memory.Priority {
		priority = "PRIORITARIA"
	}
	mmBold("PRIORIDAD  "+priority, 1300, 720, 12, managerOrange)
}

func (m *memoryManager) catalogFiltered() []int {
	kind := "official"
	if m.tab == 2 {
		kind = "swl"
	}
	query := strings.ToLower(strings.TrimSpace(m.search))
	result := []int{}
	for index, item := range m.catalog {
		if item.Kind != kind {
			continue
		}
		haystack := strings.ToLower(item.Name + " " + item.Category + " " + item.Region + " " + item.Language + " " + item.ScheduleUTC)
		if query == "" || strings.Contains(haystack, query) {
			result = append(result, index)
		}
	}
	return result
}

func (m *memoryManager) drawCatalogWorkspace() {
	if m.tab == 3 {
		mmBox(rl.Rectangle{X: 120, Y: 235, Width: 1360, Height: 520}, rl.White, managerLine)
		mmBold("CATÁLOGO NATIVO · 2026.10", 180, 295, 28, managerNavy)
		mmText("Contenido local incluido con IC-SDR; no requiere conexión y nunca modifica memorias personales.", 180, 340, 16, managerMuted)
		mmBox(rl.Rectangle{X: 180, Y: 405, Width: 1240, Height: 210}, managerPanel, managerLine)
		mmBold(fmt.Sprintf("%d ENTRADAS VERIFICADAS", len(m.catalog)), 240, 450, 19, managerBlue)
		mmText("Fuentes: ITU, NIST, NOAA, ESA y horarios publicados por las propias emisoras.", 240, 492, 15, managerInk)
		mmText("Las emisiones SWL estacionales muestran horario UTC y fecha de verificación.", 240, 527, 14, managerMuted)
		mmText("Instalar crea una copia editable; las revisiones futuras del catálogo quedan aisladas.", 240, 559, 14, managerMuted)
		return
	}
	left := rl.Rectangle{X: 20, Y: 200, Width: 255, Height: 646}
	center := rl.Rectangle{X: 290, Y: 200, Width: 970, Height: 646}
	right := rl.Rectangle{X: 1275, Y: 200, Width: 305, Height: 646}
	mmBox(left, rl.White, managerLine)
	mmBox(center, rl.White, managerLine)
	mmBox(right, rl.White, managerLine)
	title := "COLECCIONES OFICIALES"
	if m.tab == 2 {
		title = "COLECCIONES SWL"
	}
	mmBold(title, 38, 220, 13, managerMuted)
	counts := map[string]int{}
	for _, index := range m.catalogFiltered() {
		counts[m.catalog[index].Category]++
	}
	categories := make([]string, 0, len(counts))
	for category := range counts {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	mmButton(rl.Rectangle{X: 38, Y: 250, Width: 219, Height: 38}, "+ INSTALAR COLECCIÓN", managerGreen, rl.White)
	for row, category := range categories {
		y := float32(310 + row*35)
		rl.DrawCircle(44, int32(y+7), 6, m.catalogColor(category))
		mmBold(category, 58, y, 12, managerInk)
		mmText(strconv.Itoa(counts[category]), 232, y, 12, managerMuted)
	}
	indices := m.catalogFiltered()
	mmBold("CATÁLOGO", 310, 220, 13, managerMuted)
	mmText(fmt.Sprintf("%d entradas", len(indices)), 1135, 220, 12, managerMuted)
	rl.DrawRectangle(301, 249, 948, 38, managerPanel)
	headers := []string{"NOMBRE", "FRECUENCIA", "MODO", "REGIÓN", "IDIOMA", "HORARIO UTC"}
	xs := []float32{315, 625, 745, 812, 960, 1070}
	for i, h := range headers {
		mmBold(h, xs[i], 261, 11, managerMuted)
	}
	visible := 20
	m.catalogScroll = min(max(m.catalogScroll, 0), max(0, len(indices)-visible))
	end := min(len(indices), m.catalogScroll+visible)
	for row, index := range indices[m.catalogScroll:end] {
		item := m.catalog[index]
		y := float32(292 + row*26)
		if index == m.catalogSelected {
			rl.DrawRectangle(301, int32(y-4), 948, 25, rl.Color{R: 210, G: 238, B: 249, A: 255})
		}
		rl.DrawRectangle(301, int32(y-4), 4, 25, m.catalogColor(item.Category))
		mmBold(trimMemory(item.Name, 27), 315, y, 12, m.catalogColor(item.Category))
		mmText(fmt.Sprintf("%.6f", float64(item.FrequencyHz)/1e6), 625, y, 12, managerInk)
		mmText(item.Mode, 745, y, 12, managerInk)
		mmText(trimMemory(item.Region, 16), 812, y, 12, managerMuted)
		mmText(trimMemory(item.Language, 12), 960, y, 12, managerMuted)
		mmText(trimMemory(item.ScheduleUTC, 19), 1070, y, 12, managerInk)
		rl.DrawLine(306, int32(y+20), 1244, int32(y+20), rl.Color{R: 226, G: 237, B: 243, A: 255})
	}
	m.drawCatalogInspector()
}

func (m *memoryManager) catalogColor(category string) rl.Color {
	upper := strings.ToUpper(category)
	switch {
	case strings.Contains(upper, "MARINE"):
		return managerBlue
	case strings.Contains(upper, "AIR"):
		return managerOrange
	case strings.Contains(upper, "SATÉLITE"):
		return rl.Color{R: 145, G: 98, B: 205, A: 255}
	case strings.Contains(upper, "RADIO"):
		return managerGreen
	default:
		return managerCyan
	}
}

func (m *memoryManager) drawCatalogInspector() {
	mmBold("DETALLE", 1295, 220, 13, managerMuted)
	if m.catalogSelected < 0 || m.catalogSelected >= len(m.catalog) {
		mmText("Selecciona una entrada", 1300, 275, 14, managerMuted)
		mmText("para ver horario y fuente.", 1300, 301, 13, managerMuted)
		return
	}
	item := m.catalog[m.catalogSelected]
	color := m.catalogColor(item.Category)
	rl.DrawCircle(1306, 271, 7, color)
	mmBold(trimMemory(item.Name, 25), 1323, 258, 17, managerInk)
	mmText(item.Category, 1323, 282, 12, color)
	mmBold(fmt.Sprintf("%.6f MHz", float64(item.FrequencyHz)/1e6), 1298, 326, 24, managerBlue)
	mmText(item.Mode+"  ·  "+formatFilterBandwidth(item.FilterBandwidthHz), 1300, 361, 13, managerInk)
	mmBold("HORARIO UTC", 1300, 408, 11, managerMuted)
	mmText(item.ScheduleUTC, 1300, 430, 14, managerInk)
	mmBold("REGIÓN / IDIOMA", 1300, 470, 11, managerMuted)
	mmText(item.Region+" · "+item.Language, 1300, 492, 13, managerInk)
	mmText(trimMemory(item.Description, 39), 1300, 535, 12, managerMuted)
	mmText("Verificado: "+item.Verified, 1300, 578, 11, managerMuted)
	installed := m.catalogInstalled(item)
	label := "+  AÑADIR A MIS MEMORIAS"
	fill := managerGreen
	if installed {
		label = "OK  YA ESTÁ EN MIS MEMORIAS"
		fill = managerBlue
	}
	mmButton(rl.Rectangle{X: 1297, Y: 620, Width: 260, Height: 44}, label, fill, rl.White)
	mmButton(rl.Rectangle{X: 1297, Y: 676, Width: 260, Height: 38}, "SINTONIZAR SIN GUARDAR", managerPanel, managerBlue)
}

func (m *memoryManager) drawFooter() {
	rl.DrawRectangle(0, 858, 1600, 42, managerNavy)
	mmText(fmt.Sprintf("%d memorias  ·  %d grupos", len(m.memories), max(0, len(m.groups)-1)), 28, 871, 13, rl.Color{R: 190, G: 219, B: 232, A: 255})
	status := m.errorText
	if status == "" {
		status = "Sincronizado con el banco lateral"
	}
	mmText(status, 1135, 871, 13, rl.Color{R: 190, G: 219, B: 232, A: 255})
}

func (m *memoryManager) groupColor(group string) rl.Color {
	if group == "TODAS" || group == "SIN GRUPO" {
		return managerCyan
	}
	return parseColorHex(m.groupColors[group], managerBlue)
}
func memoryGroupPlain(memory MemoryEntry) string {
	if strings.TrimSpace(memory.Group) == "" {
		return "SIN GRUPO"
	}
	return memory.Group
}

func (m *memoryManager) input() {
	if m.modal != "" {
		m.modalInput()
		return
	}
	mouse := simpleui.MousePosition()
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		for i := 0; i < 4; i++ {
			if mmInside(mouse, rl.Rectangle{X: float32(28 + i*218), Y: 72, Width: 195, Height: 50}) {
				if m.tab != i {
					m.catalogSelected, m.catalogScroll = -1, 0
				}
				m.tab = i
				return
			}
		}
		if mmInside(mouse, rl.Rectangle{X: 24, Y: 137, Width: 430, Height: 36}) {
			m.searchActive = true
			return
		}
		m.searchActive = false
		if m.tab != 0 {
			if m.tab == 1 || m.tab == 2 {
				if mmInside(mouse, rl.Rectangle{X: 38, Y: 250, Width: 219, Height: 38}) {
					m.installCatalogCollection()
					return
				}
				indices := m.catalogFiltered()
				if mouse.X >= 301 && mouse.X <= 1249 && mouse.Y >= 288 && mouse.Y < 808 {
					row := int((mouse.Y-288)/26) + m.catalogScroll
					if row >= 0 && row < len(indices) {
						m.catalogSelected = indices[row]
					}
					return
				}
				if m.catalogSelected >= 0 && m.catalogSelected < len(m.catalog) {
					if mmInside(mouse, rl.Rectangle{X: 1297, Y: 620, Width: 260, Height: 44}) {
						m.installCatalog(m.catalog[m.catalogSelected])
						return
					}
					if mmInside(mouse, rl.Rectangle{X: 1297, Y: 676, Width: 260, Height: 38}) {
						m.tuneCatalog(m.catalog[m.catalogSelected])
						return
					}
				}
			}
			return
		}
		if mmInside(mouse, rl.Rectangle{X: 1370, Y: 136, Width: 205, Height: 38}) {
			m.openEditor(-1)
			return
		}
		if mmInside(mouse, rl.Rectangle{X: 38, Y: 252, Width: 219, Height: 37}) {
			m.modal = "group"
			m.buffer = ""
			m.color = 0
			return
		}
		for row, group := range m.groups[m.groupScroll:min(len(m.groups), m.groupScroll+18)] {
			if mmInside(mouse, rl.Rectangle{X: 31, Y: float32(301 + row*28), Width: 233, Height: 26}) {
				m.group = group
				m.selected = -1
				m.scroll = 0
				return
			}
		}
		indices := m.filtered()
		if mouse.X >= 301 && mouse.X <= 1249 && mouse.Y >= 288 && mouse.Y < 808 {
			row := int((mouse.Y-288)/26) + m.scroll
			if row >= 0 && row < len(indices) {
				m.selected = indices[row]
				return
			}
		}
		if m.selected >= 0 && m.selected < len(m.memories) {
			switch {
			case mmInside(mouse, rl.Rectangle{X: 1297, Y: 500, Width: 260, Height: 42}):
				m.tune()
			case mmInside(mouse, rl.Rectangle{X: 1297, Y: 552, Width: 126, Height: 38}):
				m.openEditor(m.selected)
			case mmInside(mouse, rl.Rectangle{X: 1431, Y: 552, Width: 126, Height: 38}):
				copy := m.memories[m.selected]
				copy.Name += " (copia)"
				m.memories = append(m.memories, copy)
				m.selected = len(m.memories) - 1
				m.save()
			case mmInside(mouse, rl.Rectangle{X: 1297, Y: 600, Width: 126, Height: 38}):
				m.moveSelected()
			case mmInside(mouse, rl.Rectangle{X: 1431, Y: 600, Width: 126, Height: 38}):
				m.modal = "delete"
			}
		}
	}
	if m.searchActive {
		m.inputText(&m.search, 80, false)
	}
	if m.tab == 0 {
		wheel := rl.GetMouseWheelMove()
		if wheel != 0 {
			if mouse.X < 280 {
				m.groupScroll -= int(wheel)
			} else {
				m.scroll -= int(wheel)
			}
		}
	} else if m.tab == 1 || m.tab == 2 {
		if wheel := rl.GetMouseWheelMove(); wheel != 0 {
			m.catalogScroll -= int(wheel)
		}
	}
}

func (m *memoryManager) catalogInstalled(item MemoryCatalogEntry) bool {
	for _, memory := range m.memories {
		if memory.FrequencyHz == item.FrequencyHz && strings.EqualFold(memory.Name, item.Name) {
			return true
		}
	}
	return false
}

func (m *memoryManager) catalogMemory(item MemoryCatalogEntry) MemoryEntry {
	description := item.Description
	if item.ScheduleUTC != "" {
		description += " Horario: " + item.ScheduleUTC + "."
	}
	return MemoryEntry{Name: item.Name, Description: description, FrequencyHz: item.FrequencyHz, Mode: item.Mode, FilterBandwidthHz: item.FilterBandwidthHz, StepHz: item.StepHz, ScanEnabled: true, Group: item.Category}
}

func (m *memoryManager) installCatalog(item MemoryCatalogEntry) {
	if m.catalogInstalled(item) {
		m.errorText = "Esta entrada ya está en tus memorias"
		return
	}
	m.memories = append(m.memories, m.catalogMemory(item))
	if _, exists := m.groupColors[item.Category]; !exists {
		m.groupColors[item.Category] = colorHex(m.catalogColor(item.Category))
		m.saveGroups()
	}
	m.save()
	m.rebuildGroups()
	m.errorText = "Añadida a «" + item.Category + "»"
}

func (m *memoryManager) installCatalogCollection() {
	added := 0
	for _, index := range m.catalogFiltered() {
		item := m.catalog[index]
		if m.catalogInstalled(item) {
			continue
		}
		m.memories = append(m.memories, m.catalogMemory(item))
		if _, exists := m.groupColors[item.Category]; !exists {
			m.groupColors[item.Category] = colorHex(m.catalogColor(item.Category))
		}
		added++
	}
	if added == 0 {
		m.errorText = "La colección ya está instalada"
		return
	}
	m.saveGroups()
	m.save()
	m.rebuildGroups()
	m.errorText = fmt.Sprintf("%d memorias añadidas al banco personal", added)
}

func (m *memoryManager) tuneCatalog(item MemoryCatalogEntry) {
	data, _ := json.Marshal(memoryManagerCommand{Action: "tune", ID: time.Now().UnixNano(), Memory: m.catalogMemory(item)})
	_ = os.MkdirAll(filepath.Dir(m.commandPath), 0755)
	if err := replaceLiveSnapshot(m.commandPath, data); err != nil {
		m.errorText = err.Error()
	} else {
		m.errorText = "Orden de sintonización enviada al receptor"
	}
}

func (m *memoryManager) tune() {
	data, _ := json.Marshal(memoryManagerCommand{Action: "tune", ID: time.Now().UnixNano(), Memory: m.memories[m.selected]})
	_ = os.MkdirAll(filepath.Dir(m.commandPath), 0755)
	if err := replaceLiveSnapshot(m.commandPath, data); err != nil {
		m.errorText = err.Error()
	} else {
		m.errorText = "Orden de sintonización enviada al receptor"
	}
}
func (m *memoryManager) moveSelected() {
	if len(m.groups) < 2 {
		return
	}
	current := memoryGroupPlain(m.memories[m.selected])
	pos := 0
	for i, g := range m.groups {
		if g == current {
			pos = i
		}
	}
	pos++
	if pos >= len(m.groups) {
		pos = 1
	}
	m.memories[m.selected].Group = m.groups[pos]
	m.save()
}

func (m *memoryManager) openEditor(index int) {
	m.editIndex = index
	m.field = 0
	m.errorText = ""
	if index >= 0 {
		m.edit = m.memories[index]
	} else {
		group := m.group
		if group == "TODAS" {
			group = "SIN GRUPO"
		}
		m.edit = MemoryEntry{Name: "Nueva memoria", FrequencyHz: 7_100_000, Mode: "AM", FilterBandwidthHz: 6000, StepHz: 1000, ScanEnabled: true, Group: group}
	}
	m.modal = "edit"
	m.loadBuffer()
}
func (m *memoryManager) loadBuffer() {
	switch m.field {
	case 0:
		m.buffer = m.edit.Name
	case 1:
		m.buffer = fmt.Sprintf("%.6f", float64(m.edit.FrequencyHz)/1e6)
	case 4:
		m.buffer = strconv.Itoa(m.edit.FilterBandwidthHz)
	case 5:
		m.buffer = strconv.FormatInt(m.edit.StepHz, 10)
	case 6:
		m.buffer = m.edit.Description
	default:
		m.buffer = ""
	}
}
func (m *memoryManager) storeBuffer() bool {
	v := strings.TrimSpace(m.buffer)
	switch m.field {
	case 0:
		if v == "" {
			m.errorText = "El nombre es obligatorio"
			return false
		}
		m.edit.Name = v
	case 1:
		n, e := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
		if e != nil || n <= 0 {
			m.errorText = "Frecuencia no válida"
			return false
		}
		m.edit.FrequencyHz = int64(n*1e6 + .5)
	case 4:
		n, e := strconv.Atoi(v)
		if e != nil || n <= 0 {
			m.errorText = "Filtro no válido"
			return false
		}
		m.edit.FilterBandwidthHz = n
	case 5:
		n, e := strconv.ParseInt(v, 10, 64)
		if e != nil || n <= 0 {
			m.errorText = "Paso no válido"
			return false
		}
		m.edit.StepHz = n
	case 6:
		m.edit.Description = v
	}
	m.errorText = ""
	return true
}

func (m *memoryManager) modalInput() {
	if rl.IsKeyPressed(rl.KeyEscape) {
		m.modal = ""
		return
	}
	mouse := simpleui.MousePosition()
	if m.modal == "delete" {
		if rl.IsKeyPressed(rl.KeyEnter) {
			m.deleteSelected()
		}
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
			if mmInside(mouse, rl.Rectangle{X: 830, Y: 520, Width: 190, Height: 44}) {
				m.deleteSelected()
			} else if mmInside(mouse, rl.Rectangle{X: 580, Y: 520, Width: 190, Height: 44}) {
				m.modal = ""
			}
		}
		return
	}
	if m.modal == "group" {
		m.inputText(&m.buffer, 28, false)
		if rl.IsKeyPressed(rl.KeyEnter) {
			m.commitGroup()
		}
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
			for i := range memoryGroupPalette {
				if mmInside(mouse, rl.Rectangle{X: 570 + float32(i)*58, Y: 430, Width: 42, Height: 42}) {
					m.color = i
				}
			}
			if mmInside(mouse, rl.Rectangle{X: 830, Y: 520, Width: 190, Height: 44}) {
				m.commitGroup()
			} else if mmInside(mouse, rl.Rectangle{X: 580, Y: 520, Width: 190, Height: 44}) {
				m.modal = ""
			}
		}
		return
	}
	if m.modal == "edit" {
		editable := m.field == 0 || m.field == 1 || m.field == 4 || m.field == 5 || m.field == 6
		if editable {
			m.inputText(&m.buffer, map[bool]int{true: 150, false: 40}[m.field == 6], m.field == 1 || m.field == 4 || m.field == 5)
		}
		if rl.IsKeyPressed(rl.KeyTab) && m.storeBuffer() {
			m.field = (m.field + 1) % 7
			m.loadBuffer()
		}
		if rl.IsKeyPressed(rl.KeyEnter) {
			m.commitEdit()
		}
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
			for i, r := range m.editorFields() {
				if mmInside(mouse, r) {
					if m.storeBuffer() {
						m.field = i
						if i == 2 {
							m.edit.Group = m.nextGroup(m.edit.Group)
						} else if i == 3 {
							m.edit.Mode = cycleString(m.edit.Mode, []string{"AM", "NFM", "WFM", "USB", "LSB", "CW", "DMR"})
						}
						m.loadBuffer()
					}
					return
				}
			}
			if mmInside(mouse, rl.Rectangle{X: 520, Y: 636, Width: 260, Height: 40}) {
				m.edit.ScanEnabled = !m.edit.ScanEnabled
			} else if mmInside(mouse, rl.Rectangle{X: 820, Y: 636, Width: 260, Height: 40}) {
				m.edit.Priority = !m.edit.Priority
			} else if mmInside(mouse, rl.Rectangle{X: 820, Y: 722, Width: 220, Height: 46}) {
				m.commitEdit()
			} else if mmInside(mouse, rl.Rectangle{X: 560, Y: 722, Width: 220, Height: 46}) {
				m.modal = ""
			}
		}
	}
}

func (m *memoryManager) inputText(value *string, limit int, numeric bool) {
	if rl.IsKeyPressed(rl.KeyBackspace) && len([]rune(*value)) > 0 {
		r := []rune(*value)
		*value = string(r[:len(r)-1])
	}
	for ch := rl.GetCharPressed(); ch > 0; ch = rl.GetCharPressed() {
		valid := !numeric || (ch >= '0' && ch <= '9') || ch == '.' || ch == ','
		if ch >= 32 && ch != 127 && valid && len([]rune(*value)) < limit {
			*value += string(ch)
		}
	}
}
func (m *memoryManager) nextGroup(current string) string {
	if len(m.groups) < 2 {
		return "SIN GRUPO"
	}
	for i, g := range m.groups {
		if g == current {
			return m.groups[1+(i%(len(m.groups)-1))]
		}
	}
	return m.groups[1]
}
func (m *memoryManager) commitEdit() {
	if !m.storeBuffer() {
		return
	}
	if m.editIndex >= 0 && m.editIndex < len(m.memories) {
		m.memories[m.editIndex] = m.edit
		m.selected = m.editIndex
	} else {
		m.memories = append(m.memories, m.edit)
		m.selected = len(m.memories) - 1
	}
	m.save()
	m.rebuildGroups()
	m.modal = ""
}
func (m *memoryManager) commitGroup() {
	name := strings.TrimSpace(m.buffer)
	if name == "" || strings.EqualFold(name, "TODAS") {
		m.errorText = "Escribe un nombre de grupo válido"
		return
	}
	m.groupColors[name] = colorHex(memoryGroupPalette[m.color])
	m.saveGroups()
	m.rebuildGroups()
	m.group = name
	m.modal = ""
	m.errorText = "Grupo creado"
}
func (m *memoryManager) deleteSelected() {
	if m.selected >= 0 && m.selected < len(m.memories) {
		m.memories = append(m.memories[:m.selected], m.memories[m.selected+1:]...)
		m.selected = -1
		m.save()
	}
	m.modal = ""
}

func (m *memoryManager) editorFields() []rl.Rectangle {
	return []rl.Rectangle{{X: 510, Y: 310, Width: 580, Height: 42}, {X: 510, Y: 378, Width: 280, Height: 42}, {X: 810, Y: 378, Width: 280, Height: 42}, {X: 510, Y: 446, Width: 280, Height: 42}, {X: 810, Y: 446, Width: 280, Height: 42}, {X: 510, Y: 514, Width: 280, Height: 42}, {X: 510, Y: 582, Width: 580, Height: 42}}
}
func (m *memoryManager) drawModal() {
	rl.DrawRectangle(0, 0, 1600, 900, rl.Color{R: 5, G: 25, B: 42, A: 150})
	box := rl.Rectangle{X: 450, Y: 205, Width: 700, Height: 600}
	if m.modal != "edit" {
		box = rl.Rectangle{X: 500, Y: 285, Width: 600, Height: 310}
	}
	mmBox(box, rl.White, managerLine)
	if m.modal == "delete" {
		mmBold("ELIMINAR MEMORIA", 560, 330, 22, managerRed)
		mmText("Esta acción eliminará la memoria del banco compartido.", 560, 390, 15, managerInk)
		mmButton(rl.Rectangle{X: 580, Y: 520, Width: 190, Height: 44}, "CANCELAR", managerPanel, managerInk)
		mmButton(rl.Rectangle{X: 830, Y: 520, Width: 190, Height: 44}, "ELIMINAR", managerRed, rl.White)
		return
	}
	if m.modal == "group" {
		mmBold("NUEVO GRUPO", 550, 320, 22, managerNavy)
		mmText("Nombre", 550, 360, 12, managerMuted)
		mmBox(rl.Rectangle{X: 550, Y: 382, Width: 500, Height: 40}, rl.White, managerCyan)
		mmText(m.buffer+"|", 565, 394, 14, managerInk)
		for i, c := range memoryGroupPalette {
			rl.DrawCircle(int32(591+i*58), 451, 14, c)
			if i == m.color {
				rl.DrawCircleLines(int32(591+i*58), 451, 19, managerNavy)
			}
		}
		mmButton(rl.Rectangle{X: 580, Y: 520, Width: 190, Height: 44}, "CANCELAR", managerPanel, managerInk)
		mmButton(rl.Rectangle{X: 830, Y: 520, Width: 190, Height: 44}, "CREAR GRUPO", managerBlue, rl.White)
		return
	}
	title := "EDITAR MEMORIA"
	if m.editIndex < 0 {
		title = "NUEVA MEMORIA"
	}
	mmBold(title, 500, 235, 22, managerNavy)
	labels := []string{"NOMBRE", "FRECUENCIA (MHz)", "GRUPO", "MODO", "FILTRO (Hz)", "PASO (Hz)", "DESCRIPCIÓN"}
	values := []string{m.edit.Name, fmt.Sprintf("%.6f", float64(m.edit.FrequencyHz)/1e6), m.edit.Group + "  ›", m.edit.Mode + "  ›", strconv.Itoa(m.edit.FilterBandwidthHz), strconv.FormatInt(m.edit.StepHz, 10), m.edit.Description}
	for i, r := range m.editorFields() {
		mmText(labels[i], r.X, r.Y-18, 11, managerMuted)
		border := managerLine
		if i == m.field {
			border = managerCyan
			if i == 0 || i == 1 || i == 4 || i == 5 || i == 6 {
				values[i] = m.buffer + "|"
			}
		}
		mmBox(r, managerPaper, border)
		mmText(trimMemory(values[i], 70), r.X+12, r.Y+13, 13, managerInk)
	}
	active := "OFF"
	if m.edit.ScanEnabled {
		active = "ON"
	}
	priority := "NORMAL"
	if m.edit.Priority {
		priority = "PRIORITARIA"
	}
	mmButton(rl.Rectangle{X: 520, Y: 636, Width: 260, Height: 40}, "ESCANEO  "+active, managerPanel, managerGreen)
	mmButton(rl.Rectangle{X: 820, Y: 636, Width: 260, Height: 40}, "PRIORIDAD  "+priority, managerPanel, managerOrange)
	if m.errorText != "" {
		mmText(m.errorText, 520, 690, 12, managerRed)
	}
	mmButton(rl.Rectangle{X: 560, Y: 722, Width: 220, Height: 46}, "CANCELAR", managerPanel, managerInk)
	mmButton(rl.Rectangle{X: 820, Y: 722, Width: 220, Height: 46}, "GUARDAR", managerGreen, rl.White)
}
