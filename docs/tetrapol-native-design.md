# Diseño de integración TETRAPOL liliPOL nativa

## Objetivo

Sustituir los dos decodificadores TETRAPOL existentes por un único motor liliPOL nativo x64, cargado dentro de IC-SDR y aislado del hilo gráfico. La integración no utilizará procesos auxiliares, tuberías ni puentes x86.

## Arquitectura

```text
Receiver DSP -> canal 12,5 kHz / 32 kS/s -> cola IQ continua -> worker TETRAPOL -> lilipol_native_x64.dll
                                                |-> cola de audio 8 kHz -> audio IC-SDR
                                                `-> snapshot inmutable -> panel TETRAPOL
```

Solo el worker TETRAPOL puede invocar la DLL. El receptor entrega a la DLL el canal ya filtrado y decimado a 32 kS/s, sin remuestreo intermedio. La cola aplica contrapresión si el worker se retrasa: una discontinuidad silenciosa destruye el sincronismo TETRAPOL y no es una política de descarte válida. La interfaz consume snapshots sin bloquear el procesamiento.

## Contrato del motor Go

```go
type ChannelMode uint8

const (
    ChannelCCH ChannelMode = iota
    ChannelTCH
)

type NativeConfig struct {
    Mode                 ChannelMode
    SampleRate           float64
    CorrelationThreshold float32
    TKGFilter            int
    AntaresOnly          bool
    ClearOnly            bool
    CCHCellID            string
    ChannelID            int
}

type NativeEngine interface {
    Start(NativeConfig) error
    SubmitIQ([]float32) error
    Reconfigure(NativeConfig) error
    Snapshot() NativeSnapshot
    Stop() error
}
```

`SubmitIQ` conserva el orden y la continuidad. Normalmente devuelve inmediatamente porque el motor nativo a 32 kS/s trabaja muy por encima de tiempo real; si la cola se llena espera a que haya espacio o a que `Stop` cancele la entrega. No se descartan bloques IQ silenciosamente.

## Contrato ABI de la DLL

- ABI C, convención `cdecl`.
- Proceso y DLL de 64 bits.
- IQ como `float32` entrelazado `I,Q`; `count` significa muestras complejas.
- Audio como `float32`, mono, 8 kHz.
- Todas las cadenas se intercambian como UTF-8.
- El callback de audio solo copia datos a una cola preasignada. No consulta la DLL ni toca la interfaz.
- Se añadirá `tp_get_api_version()` antes de distribuir la DLL.
- La cabecera pública debe declarar todas las funciones exportadas, incluidas las de grupos y trazas TSDU.

## Concurrencia y ciclo de vida

1. `Start` carga la DLL, comprueba la versión ABI, crea una instancia y registra el callback.
2. El worker aplica toda la configuración antes de aceptar IQ.
3. IQ, consultas de telemetría y cambios de configuración se serializan en el mismo worker.
4. Cambiar entre CCH y TCH recrea la instancia en el worker y reaplica la configuración persistente.
5. `Stop` deja de aceptar IQ, cancela las entregas bloqueadas, anula el callback, destruye la instancia y descarga la DLL.
6. `Stop` es idempotente y no depende del hilo gráfico.

No se mantendrá ningún mutex nativo mientras se llame a código de interfaz o audio de IC-SDR.

## Colas y límites

- Cola IQ: capacidad corta y fija, con contrapresión y cancelación mediante `Stop`; pérdida silenciosa prohibida.
- Cola de audio: bloques copiados en el callback y consumidos por el mezclador digital.
- Telemetría: snapshot a 10 Hz; no se consulta en cada frame gráfico.
- Historial: se copia por cambios de contador, no en cada refresco.

## Snapshot público

El panel recibe una estructura inmutable que contiene:

- estado del motor, modo y error;
- sincronización, CRC válidas/fallidas y SCR;
- AFC: corrección, error, bloqueo y fase adquisición/seguimiento;
- celda, red, país, BN, versión y listas CCH;
- sesión TCH, canal, GSSI, grupo, TKG, cifrado y antigüedad;
- audio decodificado, PLC, rechazos de correlación y bloques perdidos;
- contadores de IQ recibida y descartada.

## Estados visuales

| Estado | Presentación | Audio |
|---|---|---|
| Detenido | Gris, acción `INICIAR` | Cerrado |
| Buscando | Ámbar, actividad animada | Silencio |
| Sincronizado CCH | Verde, datos de celda | Silencio |
| Siguiendo TCH | Azul, canal y grupo | Preparado |
| Voz clara | Verde intenso, medidor de audio | Activo |
| Cifrado | Rojo, etiqueta `CIFRADO` | Bloqueado |
| Error | Rojo, mensaje concreto | Cerrado |

## Panel TETRAPOL

La zona principal se divide en cuatro pestañas:

1. **Estado**: controles CCH/TCH, accesos directos de banda, sintonía por canal, sincronización, sesión, audio y datos esenciales.
2. **Red**: celda, canales descubiertos, grupos/TKG y seguimiento CCH→TCH.
3. **Conversaciones**: historial, duración, grupo, terminal, canal y cifrado.
4. **Diagnóstico**: AFC, CRC, pérdidas IQ/audio, PLC y versión ABI.

Los controles avanzados quedan en `Diagnóstico` para que la vista principal sea operativa y clara.

### Sintonía TETRAPOL

La vista `Estado` conserva y mejora los accesos de banda actuales:

- `380–400 MHz`, `410–430 MHz` y `450–470 MHz` como botones grandes y mutuamente excluyentes;
- frecuencia actual siempre visible;
- canal anterior/siguiente con paso fijo de 12,5 kHz;
- indicación permanente `GMSK 8 kBd` y del paso activo;
- selección de banda separada del modo `CCH/TCH`, porque son decisiones distintas;
- al cambiar de banda se sintoniza una frecuencia inicial válida, se reinicia el estado de adquisición y se mantiene la configuración de audio y filtros;
- los botones quedan deshabilitados durante una transición de dispositivo SDR para evitar órdenes simultáneas.

No se añade un escáner automático dentro de estos botones: el seguimiento CCH→TCH utiliza su propio estado y la interfaz debe mostrar claramente cuándo controla la sintonía.

## Distribución

- Fuente autorizada en un directorio propio, separada del wrapper Go.
- Compilación reproducible x64 Release desde el script de distribución.
- Preferencia por runtime C/C++ estático (`/MT`) para evitar dependencias externas.
- La distribución incluye licencia o autorización del autor y versión/hash de la DLL.

## Validación necesaria

1. Prueba ABI de creación, configuración, reset y destrucción repetidos.
2. Prueba de audio con una captura IQ conocida y comparación con SDR#.
3. Cien ciclos CCH/TCH sin aumento de memoria ni hilos.
4. Cierre del programa durante recepción y durante callback de audio.
5. Saturación deliberada de la cola IQ sin congelar la interfaz.
6. Ejecución prolongada de al menos dos horas con detector de carreras habilitado en la capa Go.

## Secuencia de implementación

1. Importar y limpiar el código nativo autorizado.
2. Estabilizar la cabecera ABI y compilar x64 con `/MT`.
3. Crear pruebas de humo de la DLL.
4. Implementar loader y worker Go.
5. Conectar IQ y audio sin interfaz gráfica.
6. Validar con captura real.
7. Conectar snapshots al nuevo panel.
8. Retirar los dos decodificadores anteriores y sus recursos.
