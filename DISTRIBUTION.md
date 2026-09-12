IC-SDR Go - portable distribution
=================================

Copy the complete IC-SDR-Go folder and run IC-SDR-Go.exe while keeping DATA
next to the executable. The package includes the Visual C++ runtime required
by SoapySDR and RTL-SDR, so you do not need to install it separately.
DATA contains the SoapySDR/SDRplay, DMR, Digital Auto (DSD-neo 2.9.0), RTL_433 and APRS runtimes, as well as
their auxiliary data and licenses. It is not necessary to start the program from a
specific folder.

RADIOSONDES includes RS41, DFM and M10/M20 from rs1729/RS. Select family,
tune and press START. Their sources, GPL-3.0 license and compilation instructions
accompany the executables in DATA\tools\radiosonde\runtime.
CSV/JSON files are saved in DATA\exports\radiosonde.

AIS SHIPS integrates AIS-catcher and covers both the 161.975 and
162.025 MHz channels. The OPEN MAP button shows in another window the positions,
heading, speed and identifying data received directly by radio.

ADS-B AIRCRAFT allows selecting 1090 MHz (ADS-B/Mode S) or 978 MHz (UAT),
with an aircraft list and independent map with positions and trails.

Settings, memories, recordings, captures, exports and logs are
saved inside DATA. If the program does not show the interface, check
DATA\logs\startup.log; unrecoverable failures also display a warning.

To recreate this distribution from the source code:
    powershell -ExecutionPolicy Bypass -File .\build-release.ps1
