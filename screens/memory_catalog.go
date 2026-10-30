package screens

type MemoryCatalogEntry struct {
	ID, Kind, Category, Name, Region, Language, ScheduleUTC, Description, Source, Verified string
	FrequencyHz                                                                            int64
	Mode                                                                                   string
	FilterBandwidthHz                                                                      int
	StepHz                                                                                 int64
}

// The native catalog is read-only. Installing an item creates a personal copy,
// therefore future catalog revisions never overwrite the user's memories.
func nativeMemoryCatalog() []MemoryCatalogEntry {
	const itu = "https://www.itu.int/en/ITU-R/terrestrial/tpr/Pages/default.aspx"
	const vat = "https://www.vaticannews.va/en/short-waves.html"
	const nist = "https://www.nist.gov/pml/time-and-frequency-division/time-distribution/radio-station-wwv"
	const noaa = "https://www.noaasis.noaa.gov/POLAR/poes_overview.html"
	const esa = "https://www.esa.int/Science_Exploration/Human_and_Robotic_Exploration/International_Space_Station_Benefits_for_Humanity"
	const rri = "https://www.rri.ro/en/frequencies"
	makeEntry := func(id, kind, category, name string, hz int64, mode string, bw int, step int64, region, language, utc, description, source string) MemoryCatalogEntry {
		return MemoryCatalogEntry{ID: id, Kind: kind, Category: category, Name: name, FrequencyHz: hz, Mode: mode, FilterBandwidthHz: bw, StepHz: step, Region: region, Language: language, ScheduleUTC: utc, Description: description, Source: source, Verified: "2026-10-29"}
	}
	return []MemoryCatalogEntry{
		makeEntry("air-1215", "official", "AIR / SAR", "Emergencia aeronáutica internacional", 121_500_000, "AM", 12_000, 25_000, "Global", "Voz", "24 h", "Canal internacional de emergencia aeronáutica. Sólo recepción.", itu),
		makeEntry("air-1231", "official", "AIR / SAR", "Coordinación SAR aeronáutica", 123_100_000, "AM", 12_000, 25_000, "Global", "Voz", "Según operación", "Frecuencia internacional para búsqueda y salvamento.", itu),
		makeEntry("air-243", "official", "AIR / SAR", "Emergencia aeronáutica UHF", 243_000_000, "AM", 25_000, 25_000, "Global", "Voz", "24 h", "Frecuencia internacional de emergencia UHF. Sólo recepción.", itu),
		makeEntry("marine-ch16", "official", "MARINE / GMDSS", "Marítimo VHF canal 16", 156_800_000, "NFM", 25_000, 25_000, "Global", "Voz", "24 h", "Socorro, seguridad y llamada marítima internacional.", itu),
		makeEntry("marine-ais1", "official", "MARINE / AIS", "AIS 1", 161_975_000, "NFM", 25_000, 25_000, "Global", "Datos", "24 h", "Canal AIS 1 de identificación automática de buques.", itu),
		makeEntry("marine-ais2", "official", "MARINE / AIS", "AIS 2", 162_025_000, "NFM", 25_000, 25_000, "Global", "Datos", "24 h", "Canal AIS 2 de identificación automática de buques.", itu),
		makeEntry("gmdss-2182", "official", "MARINE / GMDSS", "GMDSS voz 2 MHz", 2_182_000, "USB", 2_700, 100, "Global", "Voz", "24 h", "Socorro y seguridad marítima MF por radiotelefonía.", itu),
		makeEntry("gmdss-4125", "official", "MARINE / GMDSS", "GMDSS voz 4 MHz", 4_125_000, "USB", 2_700, 100, "Global", "Voz", "24 h", "Socorro y seguridad marítima HF por radiotelefonía.", itu),
		makeEntry("gmdss-6215", "official", "MARINE / GMDSS", "GMDSS voz 6 MHz", 6_215_000, "USB", 2_700, 100, "Global", "Voz", "24 h", "Socorro y seguridad marítima HF por radiotelefonía.", itu),
		makeEntry("gmdss-8291", "official", "MARINE / GMDSS", "GMDSS voz 8 MHz", 8_291_000, "USB", 2_700, 100, "Global", "Voz", "24 h", "Socorro y seguridad marítima HF por radiotelefonía.", itu),
		makeEntry("gmdss-12290", "official", "MARINE / GMDSS", "GMDSS voz 12 MHz", 12_290_000, "USB", 2_700, 100, "Global", "Voz", "24 h", "Socorro y seguridad marítima HF por radiotelefonía.", itu),
		makeEntry("gmdss-16420", "official", "MARINE / GMDSS", "GMDSS voz 16 MHz", 16_420_000, "USB", 2_700, 100, "Global", "Voz", "24 h", "Socorro y seguridad marítima HF por radiotelefonía.", itu),
		makeEntry("navtex-490", "official", "MARINE / NAVTEX", "NAVTEX nacional", 490_000, "USB", 500, 100, "Región asignada", "Datos", "Programado", "Información de seguridad marítima en idioma nacional.", itu),
		makeEntry("navtex-518", "official", "MARINE / NAVTEX", "NAVTEX internacional", 518_000, "USB", 500, 100, "Global", "Inglés", "Programado", "Información internacional de seguridad marítima.", itu),
		makeEntry("noaa15", "official", "SATÉLITES METEO", "NOAA-15 APT", 137_620_000, "WFM", 40_000, 2_500, "Paso orbital", "Imagen APT", "Durante pase", "Imagen meteorológica APT; comprobar estado orbital.", noaa),
		makeEntry("noaa18", "official", "SATÉLITES METEO", "NOAA-18 APT", 137_912_500, "WFM", 40_000, 2_500, "Paso orbital", "Imagen APT", "Durante pase", "Imagen meteorológica APT; comprobar estado operativo.", noaa),
		makeEntry("noaa19", "official", "SATÉLITES METEO", "NOAA-19 APT", 137_100_000, "WFM", 40_000, 2_500, "Paso orbital", "Imagen APT", "Durante pase", "Imagen meteorológica APT; comprobar estado operativo.", noaa),
		makeEntry("iss-voice", "official", "SATÉLITES / ISS", "ISS / ARISS voz descendente", 145_800_000, "NFM", 15_000, 5_000, "Paso orbital", "Voz", "Contactos programados", "Descendente habitual de contactos educativos ARISS.", esa),
		makeEntry("wwv-2500", "swl", "SEÑALES HORARIAS", "WWV / WWVH 2.5 MHz", 2_500_000, "AM", 6_000, 100, "Global HF", "Inglés", "24 h", "Patrón de tiempo y frecuencia de NIST.", nist),
		makeEntry("wwv-5000", "swl", "SEÑALES HORARIAS", "WWV / WWVH 5 MHz", 5_000_000, "AM", 6_000, 100, "Global HF", "Inglés", "24 h", "Patrón de tiempo y frecuencia de NIST.", nist),
		makeEntry("wwv-10000", "swl", "SEÑALES HORARIAS", "WWV / WWVH 10 MHz", 10_000_000, "AM", 6_000, 100, "Global HF", "Inglés", "24 h", "Patrón de tiempo y frecuencia de NIST.", nist),
		makeEntry("wwv-15000", "swl", "SEÑALES HORARIAS", "WWV / WWVH 15 MHz", 15_000_000, "AM", 6_000, 100, "Global HF", "Inglés", "24 h", "Patrón de tiempo y frecuencia de NIST.", nist),
		makeEntry("wwv-20000", "swl", "SEÑALES HORARIAS", "WWV 20 MHz", 20_000_000, "AM", 6_000, 100, "Global HF", "Inglés", "24 h", "Patrón de tiempo y frecuencia de NIST.", nist),
		makeEntry("vr-es-7305", "swl", "RADIO INTERNACIONAL", "Vatican Radio · Español", 7_305_000, "AM", 8_000, 5_000, "Américas", "Español", "01:30–02:00 UTC", "Horario y frecuencia sujetos a temporada.", vat),
		makeEntry("vr-en-15595", "swl", "RADIO INTERNACIONAL", "Vatican Radio · English", 15_595_000, "AM", 8_000, 5_000, "Europa / Mediterráneo", "Inglés", "07:30–07:45 UTC", "Consultar días y temporada en la fuente.", vat),
		makeEntry("vr-en-11755", "swl", "RADIO INTERNACIONAL", "Vatican Radio · English", 11_755_000, "AM", 8_000, 5_000, "Europa / Mediterráneo", "Inglés", "17:15–17:30 UTC", "Frecuencia y horario sujetos a temporada.", vat),
		makeEntry("vr-af-13830", "swl", "RADIO INTERNACIONAL", "Vatican Radio · English Africa", 13_830_000, "AM", 8_000, 5_000, "África", "Inglés", "16:30–17:00 UTC", "Servicio en inglés para África.", vat),
		makeEntry("vr-af-15565", "swl", "RADIO INTERNACIONAL", "Vatican Radio · English Africa", 15_565_000, "AM", 8_000, 5_000, "África", "Inglés", "16:30–17:00 UTC", "Frecuencia paralela para África.", vat),
		makeEntry("vr-af-9705", "swl", "RADIO INTERNACIONAL", "Vatican Radio · English Africa", 9_705_000, "AM", 8_000, 5_000, "África", "Inglés", "20:00–20:30 UTC", "Servicio nocturno en inglés para África.", vat),
		makeEntry("vr-af-11870", "swl", "RADIO INTERNACIONAL", "Vatican Radio · English Africa", 11_870_000, "AM", 8_000, 5_000, "África", "Inglés", "20:00–20:30 UTC", "Frecuencia paralela nocturna para África.", vat),
		makeEntry("rri-we-9620", "swl", "RADIO INTERNACIONAL", "Radio Romania International", 9_620_000, "AM", 8_000, 5_000, "Europa Occidental", "Inglés", "06:30–07:00 UTC", "Horario de invierno 2026/2027 publicado por RRI.", rri),
		makeEntry("rri-we-15460", "swl", "RADIO INTERNACIONAL", "Radio Romania International", 15_460_000, "AM", 8_000, 5_000, "Europa Occidental", "Inglés", "12:00–13:00 UTC", "Horario de invierno 2026/2027 publicado por RRI.", rri),
		makeEntry("rri-we-7375", "swl", "RADIO INTERNACIONAL", "Radio Romania International", 7_375_000, "AM", 8_000, 5_000, "Europa Occidental", "Inglés", "21:30–22:00 UTC", "Horario de invierno 2026/2027 publicado por RRI.", rri),
		makeEntry("rri-we-7220", "swl", "RADIO INTERNACIONAL", "Radio Romania International", 7_220_000, "AM", 8_000, 5_000, "Europa Occidental", "Inglés", "23:00–00:00 UTC", "Horario de invierno 2026/2027 publicado por RRI.", rri),
		makeEntry("rri-af-21470", "swl", "RADIO INTERNACIONAL", "Radio Romania International", 21_470_000, "AM", 8_000, 5_000, "África", "Inglés", "12:00–13:00 UTC", "Horario de invierno 2026/2027 publicado por RRI.", rri),
		makeEntry("rri-na-7250", "swl", "RADIO INTERNACIONAL", "Radio Romania International", 7_250_000, "AM", 8_000, 5_000, "Norteamérica Este", "Inglés", "21:30–22:00 UTC", "Horario de invierno 2026/2027 publicado por RRI.", rri),
		makeEntry("rri-na-7325", "swl", "RADIO INTERNACIONAL", "Radio Romania International", 7_325_000, "AM", 8_000, 5_000, "Norteamérica Este", "Inglés", "01:00–02:00 UTC", "Horario de invierno 2026/2027 publicado por RRI.", rri),
		makeEntry("rri-na-7220", "swl", "RADIO INTERNACIONAL", "Radio Romania International", 7_220_000, "AM", 8_000, 5_000, "Norteamérica Oeste", "Inglés", "04:00–05:00 UTC", "Horario de invierno 2026/2027 publicado por RRI.", rri),
		makeEntry("rri-jp-9620", "swl", "RADIO INTERNACIONAL", "Radio Romania International", 9_620_000, "AM", 8_000, 5_000, "Japón", "Inglés", "23:00–00:00 UTC", "Horario de invierno 2026/2027 publicado por RRI.", rri),
	}
}
