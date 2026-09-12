package bandplan

func init() { register(ituR2()) }

func ituR2() Plan {
	plan := clone(ituR1(), "itu-r2", "ITU Region 2 · Americas", "R2")
	plan.APRSHz = 144_390_000
	replaceRange(&plan, "HAM", "80 m", 3_500_000, 4_000_000)
	replaceRange(&plan, "HAM", "40 m", 7_000_000, 7_300_000)
	replaceRange(&plan, "HAM", "6 m", 50_000_000, 54_000_000)
	replaceRange(&plan, "HAM", "2 m", 144_000_000, 148_000_000)
	replaceRange(&plan, "HAM", "70 cm", 420_000_000, 450_000_000)
	removeRange(&plan, "HAM", "4 m")
	replaceRange(&plan, "HAM", "1.25 m", 222_000_000, 225_000_000)
	replaceRange(&plan, "HAM", "33 cm", 902_000_000, 928_000_000)
	replaceRange(&plan, "COMMERCIAL", "FM", 88_000_000, 108_000_000)
	replaceRange(&plan, "COMMERCIAL", "MW / AM", 530_000, 1_700_000)
	removeRange(&plan, "ISM", "PMR446")
	removeRange(&plan, "ISM", "868 MHz")
	replaceRange(&plan, "ISM", "FRS / GMRS", 462_500_000, 467_700_000)
	replaceRange(&plan, "ISM", "MURS", 151_820_000, 154_600_000)

	replaceTune(&plan, "HAM", "80 m", 3_900_000, 50_000)
	replaceTune(&plan, "HAM", "40 m", 7_200_000, 50_000)
	replaceTune(&plan, "HAM", "6 m", 50_125_000, 200_000)
	replaceTune(&plan, "HAM", "2 m", 146_520_000, 500_000)
	replaceTune(&plan, "HAM", "70 cm", 446_000_000, 500_000)
	removeTune(&plan, "HAM", "4 m")
	replaceTune(&plan, "HAM", "1.25 m", 223_500_000, 200_000)
	replaceTune(&plan, "HAM", "33 cm", 915_000_000, 1_000_000)
	replaceTune(&plan, "COMMERCIAL", "FM", 98_100_000, 2_000_000)
	replaceTune(&plan, "COMMERCIAL", "MW / AM", 1_000_000, 500_000)
	removeTune(&plan, "ISM", "PMR446")
	removeTune(&plan, "ISM", "868 MHz")
	replaceTune(&plan, "ISM", "FRS / GMRS", 462_562_500, 500_000)
	replaceTune(&plan, "ISM", "MURS", 154_570_000, 250_000)
	return plan
}
