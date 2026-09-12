package bandplan

func init() {
	register(countryUS())
	register(countryCA())
	register(countryMX())
	register(countryBR())
	register(countryAR())
	register(countryAU())
	register(countryNZ())
	register(countryJP())
	register(countryUK())
	register(countryDE())
	register(countryES())
	register(countryFR())
	register(countryIT())
	register(countryPL())
	register(countryZA())
	register(countryRU())
	register(countryIN())
	register(countryCN())
	register(countryKR())
	register(countryID())
	register(countryPH())
}

func countryUS() Plan {
	plan := clone(ituR2(), "us", "United States", "R2")
	plan.APRSHz = 144_390_000
	plan.Default = Tune{Category: "HAM", Name: "20 m", FrequencyHz: 14_261_000, SpanHz: 100_000}
	applyNorthAmericanPersonalRadio(&plan)
	return plan
}

func countryCA() Plan {
	plan := clone(ituR2(), "ca", "Canada", "R2")
	plan.APRSHz = 144_390_000
	applyNorthAmericanPersonalRadio(&plan)
	return plan
}

func countryMX() Plan {
	plan := clone(ituR2(), "mx", "Mexico", "R2")
	plan.APRSHz = 144_390_000
	replaceRange(&plan, "ISM", "FRS / GMRS", 462_500_000, 467_700_000)
	replaceTune(&plan, "ISM", "FRS / GMRS", 462_562_500, 500_000)
	return plan
}

func countryBR() Plan {
	plan := clone(ituR2(), "br", "Brazil", "R2")
	plan.APRSHz = 144_390_000
	replaceRange(&plan, "HAM", "70 cm", 430_000_000, 440_000_000)
	replaceTune(&plan, "HAM", "70 cm", 433_500_000, 500_000)
	return plan
}

func countryAR() Plan {
	plan := clone(ituR2(), "ar", "Argentina", "R2")
	plan.APRSHz = 144_390_000
	return plan
}

func countryAU() Plan {
	plan := clone(ituR3(), "au", "Australia", "R3")
	plan.APRSHz = 145_175_000
	replaceRange(&plan, "HAM", "80 m", 3_500_000, 3_700_000)
	replaceRange(&plan, "HAM", "70 cm", 420_000_000, 450_000_000)
	replaceTune(&plan, "HAM", "2 m", 146_500_000, 500_000)
	applyUHFCB(&plan)
	return plan
}

func countryNZ() Plan {
	plan := clone(ituR3(), "nz", "New Zealand", "R3")
	plan.APRSHz = 144_575_000
	replaceRange(&plan, "HAM", "80 m", 3_500_000, 3_900_000)
	applyUHFCB(&plan)
	return plan
}

func countryJP() Plan {
	plan := clone(ituR3(), "jp", "Japan", "R3")
	plan.APRSHz = 144_660_000
	replaceRange(&plan, "HAM", "2 m", 144_000_000, 146_000_000)
	replaceRange(&plan, "HAM", "70 cm", 430_000_000, 440_000_000)
	removeRange(&plan, "ISM", "UHF CB")
	removeTune(&plan, "ISM", "UHF CB")
	replaceTune(&plan, "HAM", "2 m", 145_000_000, 500_000)
	return plan
}

func countryUK() Plan {
	plan := clone(ituR1(), "uk", "United Kingdom", "R1")
	plan.APRSHz = 144_800_000
	replaceRange(&plan, "HAM", "4 m", 70_000_000, 70_500_000)
	return plan
}

func countryDE() Plan {
	plan := clone(ituR1(), "de", "Germany", "R1")
	plan.APRSHz = 144_800_000
	return plan
}

func countryES() Plan {
	plan := clone(ituR1(), "es", "Spain", "R1")
	plan.APRSHz = 144_800_000
	return plan
}

func countryFR() Plan {
	plan := clone(ituR1(), "fr", "France", "R1")
	plan.APRSHz = 144_800_000
	return plan
}

func countryIT() Plan {
	plan := clone(ituR1(), "it", "Italy", "R1")
	plan.APRSHz = 144_800_000
	return plan
}

func countryPL() Plan {
	plan := clone(ituR1(), "pl", "Poland", "R1")
	plan.APRSHz = 144_800_000
	return plan
}

func countryZA() Plan {
	plan := clone(ituR1(), "za", "South Africa", "R1")
	plan.APRSHz = 144_800_000
	replaceRange(&plan, "HAM", "2 m", 144_000_000, 146_000_000)
	return plan
}

func countryRU() Plan {
	plan := clone(ituR1(), "ru", "Russia", "R1")
	plan.APRSHz = 144_800_000
	replaceRange(&plan, "HAM", "2 m", 144_000_000, 146_000_000)
	return plan
}

func countryIN() Plan {
	plan := clone(ituR3(), "in", "India", "R3")
	plan.APRSHz = 144_800_000
	replaceRange(&plan, "HAM", "2 m", 144_000_000, 146_000_000)
	replaceRange(&plan, "HAM", "70 cm", 434_000_000, 438_000_000)
	removeRange(&plan, "ISM", "UHF CB")
	removeTune(&plan, "ISM", "UHF CB")
	return plan
}

func countryCN() Plan {
	plan := clone(ituR3(), "cn", "China", "R3")
	plan.APRSHz = 144_640_000
	replaceRange(&plan, "HAM", "2 m", 144_000_000, 148_000_000)
	replaceRange(&plan, "HAM", "70 cm", 430_000_000, 440_000_000)
	removeRange(&plan, "ISM", "UHF CB")
	removeTune(&plan, "ISM", "UHF CB")
	return plan
}

func countryKR() Plan {
	plan := clone(ituR3(), "kr", "South Korea", "R3")
	plan.APRSHz = 144_620_000
	replaceRange(&plan, "HAM", "2 m", 144_000_000, 146_000_000)
	removeRange(&plan, "ISM", "UHF CB")
	removeTune(&plan, "ISM", "UHF CB")
	return plan
}

func countryID() Plan {
	plan := clone(ituR3(), "id", "Indonesia", "R3")
	plan.APRSHz = 144_390_000
	return plan
}

func countryPH() Plan {
	plan := clone(ituR3(), "ph", "Philippines", "R3")
	plan.APRSHz = 144_390_000
	replaceRange(&plan, "HAM", "2 m", 144_000_000, 148_000_000)
	return plan
}

func applyNorthAmericanPersonalRadio(plan *Plan) {
	replaceRange(plan, "ISM", "FRS / GMRS", 462_500_000, 467_700_000)
	replaceRange(plan, "ISM", "MURS", 151_820_000, 154_600_000)
	replaceTune(plan, "ISM", "FRS / GMRS", 462_562_500, 500_000)
	replaceTune(plan, "ISM", "MURS", 154_570_000, 250_000)
}

func applyUHFCB(plan *Plan) {
	replaceRange(plan, "ISM", "UHF CB", 476_412_500, 477_412_500)
	replaceTune(plan, "ISM", "UHF CB", 476_625_000, 500_000)
}
