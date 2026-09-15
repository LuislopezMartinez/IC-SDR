# Idiomas de IC-SDR

El programa usa español por defecto. Al arrancar, busca los archivos `*.json` de esta carpeta, situada junto al ejecutable. El menú muestra cada idioma válido encontrado. La elección se guarda en `DATA/config/language.json`.

Para crear una traducción:

1. Copia `es.json` con otro nombre, por ejemplo `fr.json`.
2. Cambia `id` a un identificador único (`fr`), `name` al nombre nativo (`Français`) y `flag` a un código de país (`FR`). España (`ES`) y Reino Unido (`GB`) tienen banderas dibujadas; otros códigos aparecen como texto.
3. Traduce los valores de `texts`. Conserva las claves `text.…`: son identificadores estables compartidos por todos los idiomas.
4. Conserva los formatos `%s`, `%d`, `%.3f`, etc., en el mismo orden. Representan datos que el programa inserta. No traduzcas indicativos, nombres de protocolos ni unidades.
5. Guarda como JSON UTF-8 y reinicia el programa. Si falta una clave, se muestra el texto español. Un archivo mal formado o con formatos incompatibles se ignora.

Los textos de presentación se traducen sin modificar los identificadores internos, las frecuencias ni los mensajes originales recibidos por radio. Los visores nuevos usan la selección guardada al abrirse.

Los archivos de español e inglés también se incluyen en el ejecutable como respaldo si falta esta carpeta. Los archivos externos permiten añadir o corregir traducciones sin recompilar.
