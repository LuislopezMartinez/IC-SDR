# IC-SDR

**Multimode SDR for Windows, written in Go for maximum efficiency.**

**Current version: [v0.3.1](https://github.com/LuislopezMartinez/IC-SDR/releases/tag/v0.3.1)**

IC-SDR brings together reception, demodulation, spectrum analysis and digital signal decoding in a desktop interface designed for daily use.

> [!IMPORTANT]
> IC-SDR is designed specifically for **Windows**. The binary and all components required for portable distribution are located in the `dist/IC-SDR-Go` folder after generating the package.

![IC-SDR main interface](docs/images/ic-sdr-principal.png)

## Features

- Demodulation in **AM, NFM, WFM, LSB and USB**.
- Support for digital modes.
- Real-time spectrum and waterfall.
- Memory bank organized by groups.
- Audio recorder with automatic blank space removal.
- Frequency segment scanner with instant trigger.
- Tone detector and squelch control.
- Five-band equalizer and audio processing controls.

## Decoders

IC-SDR integrates tools to receive and display:

- **AIS** — vessel tracking on maritime channels.
- **ADS-B** — aircraft reception on 1090 MHz and UAT 978 MHz.
- **Radiosondes** — compatibility with RS41, DFM and M10/M20.
- **APRS** — packet reception and display.
- **RTL_433** — decoding of sensors and ISM devices, with CSV export.
- **DMR** — digital radio reception.
- **SSTV** — slow-scan television.
- **TETRA** — TETRA signal reception and analysis.

![RTL_433 decoding in IC-SDR](docs/images/ic-sdr-rtl433.png)

## What's new in v0.3.1

- New satellite tracking module with TLE catalog updated from CelesTrak.
- World map with position, orbit, visibility and satellite details.
- Search, grouping and tuning of frequencies associated with each satellite.
- Audio recording in **MP3 or WAV**, selectable from the interface.
- Redesigned recorder with level meter, history, playback and file deletion.
- Improvements in automatic silence skipping via squelch.
- Visual and usability adjustments in memories, scanner and tools menu.
- New tests for satellites, recording and memory markers.

## What's new in v0.2.1

- New visual themes and improvements in contrast and readability throughout the interface.
- Expanded memory management with descriptions, priorities, colors and group editing.
- New presets for aeronautical, maritime and ISS/ARISS bands.
- Improvements in automatic SSTV mode and candidate mode selection.
- Redesign and usability adjustments in audio, scanner, recorder and utilities panels.
- New tests for themes, contrast, memories and SSTV.

## Windows and portable distribution

IC-SDR is intended to run on Windows. The local `dist/IC-SDR-Go` folder contains the distributable binary `IC-SDR-Go.exe`, its runtimes and the necessary auxiliary tools. The `DATA` directory must remain next to the executable.

The `dist/` folder is generated locally and is not part of the versioned source code. To rebuild it, use `build-release.ps1`.

## Requirements

- Windows.
- Go 1.22 or later to compile from source (see `go.mod`).
- A receiver compatible with RTL-SDR or SoapySDR/SDRplay.

## Compilation

From the repository root:

```powershell
go build .
```

To generate the portable Windows distribution:

```powershell
powershell -ExecutionPolicy Bypass -File .\build-release.ps1
```

The distribution is created in `dist/IC-SDR-Go`. See [DISTRIBUTION.md](DISTRIBUTION.md) for more information about the portable package and data directories.

## Data and configuration

Settings, memories, recordings, captures, exports and logs are stored under `DATA`. Data generated during use is not included in the repository.

## Project status

IC-SDR is under active development. Available features may vary depending on the receiver, drivers and installed decoding tools.

This tree is an English-language fork of [LuislopezMartinez/IC-SDR](https://github.com/LuislopezMartinez/IC-SDR). UI, logs, decoder status, default memories, docs, and tests are in English. Old Spanish settings files (`PAQUETES`, `SONDAS`, `SIN GRUPO`, and similar) are still accepted and mapped on load.
