package bandplan

func init() { register(ituR3()) }

func ituR3() Plan {
	plan := clone(ituR1(), "itu-r3", "ITU Region 3 · Asia / Pacific", "R3")
	plan.APRSHz = 144_800_000
	replaceRange(&plan, "HAM", "80 m", 3_500_000, 3_900_000)
	replaceRange(&plan, "HAM", "40 m", 7_000_000, 7_300_000)
	replaceRange(&plan, "HAM", "6 m", 50_000_000, 54_000_000)
	replaceRange(&plan, "HAM", "2 m", 144_000_000, 148_000_000)
	replaceRange(&plan, "HAM", "70 cm", 430_000_000, 450_000_000)
	removeRange(&plan, "HAM", "4 m")
	replaceRange(&plan, "COMMERCIAL", "FM", 87_500_000, 108_000_000)
	removeRange(&plan, "ISM", "PMR446")
	removeTune(&plan, "HAM", "4 m")
	replaceTune(&plan, "HAM", "80 m", 3_600_000, 50_000)
	replaceTune(&plan, "HAM", "40 m", 7_150_000, 50_000)
	replaceTune(&plan, "HAM", "6 m", 50_110_000, 200_000)
	replaceTune(&plan, "HAM", "2 m", 146_500_000, 500_000)
	replaceTune(&plan, "HAM", "70 cm", 439_000_000, 500_000)
	removeTune(&plan, "ISM", "PMR446")
	return plan
}
