# Captura IQ TETRAPOL para Wavecom W-CODE

IC-SDR genera una captura por canal en `DATA/captures/tetrapol`. Cada sesión contiene:

- `*.wav`: IQ complejo, PCM firmado de 16 bits, estéreo, 128000 muestras/s.
- `*.wav.json`: frecuencia central del SDR, frecuencia del canal y formato.

El canal izquierdo del WAV es **I** y el derecho **Q**. La captura ya desplaza
el canal seleccionado a 0 Hz y aplica filtrado anti-alias antes de remuestrear.

## W-CODE

1. En IC-SDR, abra **TETRA** y sintonice el canal TETRAPOL autorizado.
2. Pulse **CAPTURAR IQ WAVECOM**; pulse de nuevo al terminar.
3. En W-CODE, cree una entrada personalizada de tipo archivo y seleccione el
   WAV generado.
4. Seleccione formato **I/Q**, dos canales, PCM de 16 bits y 128000 Hz.
5. Active el modo **TETRAPOL**, ancho de banda 12,5 kHz y ajuste el offset a
   0 Hz. Consulte el JSON para confirmar qué frecuencia física representa la
   grabación.
6. Si no hay sincronismo en un downlink con buena señal, pruebe la polaridad
   I/Q inversa: la especificación TETRAPOL normalmente transmite el downlink
   como INV, aunque algunos receptores ya lo invierten antes de la captura.

La captura sólo conserva IQ de radio; no intenta descifrar ni transformar voz.
Úsela únicamente para señales y grabaciones cuya recepción esté autorizada.
