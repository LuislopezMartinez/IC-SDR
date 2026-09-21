IC-SDR Go - distribución portable
=================================

Copie la carpeta IC-SDR-Go completa y ejecute IC-SDR-Go.exe conservando DATA
junto al ejecutable. El paquete incluye el runtime de Visual C++ que requieren
SoapySDR y RTL-SDR, por lo que no necesita instalarlo por separado.
DATA contiene los runtimes de SoapySDR/SDRplay, DMR, Digital Auto (DSD-neo 2.9.0), RTL_433 y APRS, así como
sus datos auxiliares y licencias. No es necesario iniciar el programa desde una
carpeta concreta.

El control CAT incluye OmniRig 1.20 y los perfiles de radio oficiales. Al
activar RIG, IC-SDR utiliza una instalación existente si funciona; en caso
contrario activa la copia incluida únicamente para el usuario actual, sin
instalador ni permisos de administrador. La primera vez se abre la ventana de
OmniRig para configurar RIG 1, puerto COM, velocidad y modelo de radio.
El tool OMNIRIG CAT mantiene esa ventana oculta durante el uso normal y reúne
el estado, la frecuencia, el modo y los tres modos de enlace de IC-SDR. El
botón CONFIG. AVANZADA muestra temporalmente la ventana original para los
ajustes de puerto y perfil que OmniRig no ofrece mediante su interfaz COM.
Los selectores RIG 1 y RIG 2 muestran el modelo configurado y permiten alternar
entre ambas radios sin transferir automáticamente la sintonía anterior.
El switch MUTE AUDIO EN TX puede silenciar el audio local cuando el RIG elegido
confirma transmisión por CAT. Durante ese silencio, la cabecera muestra en rojo
RIG → MUTE; al volver a RX se restaura el estado manual anterior.

RADIOSONDAS incluye RS41, DFM y M10/M20 de rs1729/RS. Seleccione familia,
sintonice y pulse INICIAR. Sus fuentes, licencia GPL-3.0 e instrucciones de
compilación acompañan a los ejecutables en DATA\tools\radiosonde\runtime.
Los CSV/JSON se guardan en DATA\exports\radiosonde.

TETRAPOL incluye tetrapol_dump y el códec RP-CELP compilados como ejecutables
externos, sus DLL y las fuentes GPL correspondientes. Solo se reproducen
llamadas que la capa de protocolo identifique explícitamente como no cifradas.

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
