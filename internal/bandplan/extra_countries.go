package bandplan

func init() { registerExtraCountries() }

func registerExtraCountries() {
	type spec struct{ id, name, itu string }
	for _, item := range []spec{
		{"nl", "Netherlands", "R1"}, {"be", "Belgium", "R1"}, {"at", "Austria", "R1"},
		{"ch", "Switzerland", "R1"}, {"ie", "Ireland", "R1"}, {"pt", "Portugal", "R1"},
		{"se", "Sweden", "R1"}, {"no", "Norway", "R1"}, {"fi", "Finland", "R1"},
		{"dk", "Denmark", "R1"}, {"cz", "Czechia", "R1"}, {"hu", "Hungary", "R1"},
		{"ro", "Romania", "R1"}, {"tr", "Turkey", "R1"}, {"ua", "Ukraine", "R1"},
		{"gr", "Greece", "R1"}, {"bg", "Bulgaria", "R1"},
		{"cl", "Chile", "R2"}, {"co", "Colombia", "R2"}, {"pe", "Peru", "R2"},
		{"ve", "Venezuela", "R2"}, {"uy", "Uruguay", "R2"},
		{"sg", "Singapore", "R3"}, {"my", "Malaysia", "R3"}, {"tw", "Taiwan", "R3"},
		{"vn", "Vietnam", "R3"}, {"th", "Thailand", "R3"}, {"hk", "Hong Kong", "R3"},
	} {
		base := ituR1()
		switch item.itu {
		case "R2":
			base = ituR2()
		case "R3":
			base = ituR3()
		}
		register(clone(base, item.id, item.name, item.itu))
	}
}
