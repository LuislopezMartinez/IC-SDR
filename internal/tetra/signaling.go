package tetra

import (
	"fmt"
	"sort"
	"strings"
)

type fieldRule struct {
	Name        string
	Bits        int
	Type, Ref   string
	Value, Skip int
}

func parseFields(bits []byte, off int, rules []fieldRule) (map[string]uint32, int, bool) {
	values := map[string]uint32{}
	skip := 0
	for _, r := range rules {
		if skip > 0 {
			skip--
			continue
		}
		switch r.Type {
		case "Switch":
			if values[r.Ref] != uint32(r.Value) {
				continue
			}
		case "SwitchNot":
			if values[r.Ref] == uint32(r.Value) {
				continue
			}
		case "Jump", "JumpNot":
			hit := values[r.Ref] == uint32(r.Value)
			if (r.Type == "Jump" && hit) || (r.Type == "JumpNot" && !hit) {
				skip = r.Skip
			}
			continue
		}
		if off+r.Bits > len(bits) {
			return values, off, false
		}
		v := bitsToUint(bits, off, r.Bits)
		off += r.Bits
		switch r.Type {
		case "Options_bit":
			if v == 0 {
				return values, off, true
			}
		case "Presence_bit":
			if v == 0 {
				skip = r.Skip
			}
		case "More_bit":
			return values, off, true
		default:
			if r.Name != "Reserved" {
				values[r.Name] = v
			}
		}
	}
	return values, off, true
}
func enrichCMCE(tl []byte, c *cmceInfo) bool {
	var rules []fieldRule
	switch c.Code {
	case 2:
		rules = connectRules
	case 5:
		rules = infoRules
	case 6:
		rules = releaseRules
	case 7:
		rules = setupRules
	case 9:
		rules = ceasedRules
	case 11:
		rules = grantRules
	default:
		return true
	}
	fields, _, ok := parseFields(tl, 8, rules)
	if !ok {
		return false
	}
	c.Fields = fields
	if v, ok := fields["Calling_party_address_SSI"]; ok {
		c.CallingSSI = v
	}
	if v, ok := fields["Transmitting_party_address_SSI"]; ok {
		c.CallingSSI = v
	}
	return true
}
func (m Message) Detail() string {
	s := fmt.Sprintf("SSI:%d", m.AddressSSI)
	if m.Carrier != nil {
		s = fmt.Sprintf("Carrier:%d AssignedSlotsMask:%d RX_TS:%d ", *m.Carrier, m.AssignedSlots, m.Slot) + s
	}
	if m.CallID != 0 {
		s += fmt.Sprintf(" Call ID:%d", m.CallID)
	}
	s += " " + m.Kind
	if v, ok := m.Fields["Transmission_grant"]; ok {
		s += " Transmission " + []string{"Granted", "Not_granted", "Request_queued", "Granted_to_another_user"}[v&3]
	}
	if m.PartySSI != 0 {
		s += fmt.Sprintf(" Party_SSI:%d", m.PartySSI)
	}
	if v, ok := m.Fields["Basic_service_Communication_type"]; ok {
		s += " Basic_service:" + []string{"Indiv", "Group", "P2MP", "Broadcast"}[v&3]
	}
	if v, ok := m.Fields["Basic_service_Encryption_flag"]; ok {
		if v == 0 {
			s += " Clear"
		} else {
			s += " E2EE"
		}
	}
	if v, ok := m.Fields["Basic_service_Circuit_mode_type"]; ok {
		s += " " + []string{"Speech_TCH_S", "Unprotect_72", "Low_Protect_48_1", "Low_Protect_48_4", "Low_Protect_48_8", "High_Protect_24_1", "High_Protect_24_4", "High_Protect_24_8"}[v&7]
	}
	if m.Text != "" && m.Text != m.Kind {
		s += " · " + m.Text
	}
	keys := make([]string, 0, len(m.Fields))
	for k := range m.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s += fmt.Sprintf(" %s:%d", k, m.Fields[k])
	}
	if m.RawHex != "" {
		s += fmt.Sprintf(" RAW[%d bits]:%s", m.RawBits, m.RawHex)
	}
	return strings.TrimSpace(s)
}
