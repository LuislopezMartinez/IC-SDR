# Maps

Live ADS-B and AIS maps use Web Mercator raster tiles with city, region, and
street labels. No API key is required.

Primary: OpenStreetMap Carto from the German community tile servers
(`https://tile.openstreetmap.de/{z}/{x}/{y}.png`).

Fallback: Esri World Street Map
(`https://server.arcgisonline.com/ArcGIS/rest/services/World_Street_Map/MapServer/tile/{z}/{y}/{x}`).

CARTO Voyager is not used. Their public CDN now returns watermarked tiles that
read "API KEY REQUIRED".

Tiles are cached under `DATA/cache/map-tiles-v2/{z}/{x}/{y}.tile`. Requests send
`User-Agent: IC-SDR/0.5 (+https://github.com/blkph0x/IC-SDR)`.

Attribution shown on the maps: © OpenStreetMap · Esri.

`ais-world.png` remains as an offline fallback at wide zoom. It was rendered
from Natural Earth `ne_50m_admin_0_countries.geojson` (public domain):
https://www.naturalearthdata.com/about/terms-of-use/
Source: https://github.com/nvkelso/natural-earth-vector
