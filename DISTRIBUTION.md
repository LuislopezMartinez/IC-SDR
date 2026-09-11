IC-SDR Go - distribución portable
=================================

Copie la carpeta IC-SDR-Go completa y ejecute IC-SDR-Go.exe conservando DATA
junto al ejecutable. El paquete incluye el runtime de Visual C++ que requieren
SoapySDR y RTL-SDR, por lo que no necesita instalarlo por separado.
DATA contiene los runtimes de SoapySDR/SDRplay, DMR, Digital Auto (DSD-neo 2.9.0), RTL_433 y APRS, así como
sus datos auxiliares y licencias. No es necesario iniciar el programa desde una
carpeta concreta.

RADIOSONDAS incluye RS41, DFM y M10/M20 de rs1729/RS. Seleccione familia,
sintonice y pulse INICIAR. Sus fuentes, licencia GPL-3.0 e instrucciones de
compilación acompañan a los ejecutables en DATA\tools\radiosonde\runtime.
Los CSV/JSON se guardan en DATA\exports\radiosonde.

AIS BARCOS integra AIS-catcher y cubre a la vez los canales de 161,975 y
162,025 MHz. El botón ABRIR MAPA presenta en otra ventana las posiciones,
rumbo, velocidad y datos identificativos recibidos directamente por radio.

ADS-B AVIONES permite seleccionar 1090 MHz (ADS-B/Mode S) o 978 MHz (UAT),
con lista de aeronaves y mapa independiente con posiciones y estelas.

Los ajustes, memorias, grabaciones, capturas, exportaciones y registros se
guardan dentro de DATA. Si el programa no llega a mostrar la interfaz, consulte
DATA\logs\startup.log; los fallos irrecuperables también muestran un aviso.

Para crear de nuevo esta distribución desde el código fuente:
    powershell -ExecutionPolicy Bypass -File .\build-release.ps1
